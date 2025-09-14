package sub

import (
	"context"
	"crypto/tls"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"x-ui/logger"
	"x-ui/util/common"
	webpkg "x-ui/web"
	"x-ui/web/locale"
	"x-ui/web/middleware"
	"x-ui/web/network"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

// setEmbeddedTemplates parses and sets embedded templates on the engine
func setEmbeddedTemplates(engine *gin.Engine) error {
	t, err := template.New("").Funcs(engine.FuncMap).ParseFS(
		webpkg.EmbeddedHTML(),
		"html/common/page.html",
		"html/component/aThemeSwitch.html",
		"html/subscription.html",
	)
	if err != nil {
		return err
	}
	engine.SetHTMLTemplate(t)
	return nil
}

type Server struct {
	httpServer *http.Server
	listener   net.Listener

	sub            *SUBController
	settingService service.SettingService

	ctx    context.Context
	cancel context.CancelFunc
}

func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Server) initRouter() (*gin.Engine, error) {
	// Always run in release mode for the subscription server
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	gin.SetMode(gin.ReleaseMode)

	engine := gin.Default()

	subDomain, err := s.settingService.GetSubDomain()
	if err != nil {
		return nil, err
	}

	if subDomain != "" {
		engine.Use(middleware.DomainValidatorMiddleware(subDomain))
	}

	// Provide base_path in context for templates
	engine.Use(func(c *gin.Context) {
		c.Set("base_path", "/")
	})

	LinksPath, err := s.settingService.GetSubPath()
	if err != nil {
		return nil, err
	}

	JsonPath, err := s.settingService.GetSubJsonPath()
	if err != nil {
		return nil, err
	}

	Encrypt, err := s.settingService.GetSubEncrypt()
	if err != nil {
		return nil, err
	}

	ShowInfo, err := s.settingService.GetSubShowInfo()
	if err != nil {
		return nil, err
	}

	RemarkModel, err := s.settingService.GetRemarkModel()
	if err != nil {
		RemarkModel = "-ieo"
	}

	SubUpdates, err := s.settingService.GetSubUpdates()
	if err != nil {
		SubUpdates = "10"
	}

	SubJsonFragment, err := s.settingService.GetSubJsonFragment()
	if err != nil {
		SubJsonFragment = ""
	}

	SubJsonNoises, err := s.settingService.GetSubJsonNoises()
	if err != nil {
		SubJsonNoises = ""
	}

	SubJsonMux, err := s.settingService.GetSubJsonMux()
	if err != nil {
		SubJsonMux = ""
	}

	SubJsonRules, err := s.settingService.GetSubJsonRules()
	if err != nil {
		SubJsonRules = ""
	}

	SubTitle, err := s.settingService.GetSubTitle()
	if err != nil {
		SubTitle = ""
	}

	// set per-request localizer from headers/cookies
	engine.Use(locale.LocalizerMiddleware())

	// register i18n function similar to web server
	i18nWebFunc := func(key string, params ...string) string {
		return locale.I18n(locale.Web, key, params...)
	}
	engine.SetFuncMap(map[string]any{"i18n": i18nWebFunc})

	// Templates: prefer embedded; fallback to disk if necessary
	if err := setEmbeddedTemplates(engine); err != nil {
		logger.Warning("sub: failed to parse embedded templates:", err)
		if files, derr := s.getHtmlFiles(); derr == nil {
			engine.LoadHTMLFiles(files...)
		} else {
			logger.Error("sub: no templates available (embedded parse and disk load failed)", err, derr)
		}
	}

	// Assets: use disk if present, fallback to embedded
	if _, err := os.Stat("web/assets"); err == nil {
		engine.StaticFS("/assets", http.FS(os.DirFS("web/assets")))
	} else {
		if subFS, err := fs.Sub(webpkg.EmbeddedAssets(), "assets"); err == nil {
			engine.StaticFS("/assets", http.FS(subFS))
		} else {
			logger.Error("sub: failed to mount embedded assets:", err)
		}
	}

	g := engine.Group("/")

	s.sub = NewSUBController(
		g, LinksPath, JsonPath, Encrypt, ShowInfo, RemarkModel, SubUpdates,
		SubJsonFragment, SubJsonNoises, SubJsonMux, SubJsonRules, SubTitle)

	return engine, nil
}

// getHtmlFiles loads templates from local folder (used in debug mode)
func (s *Server) getHtmlFiles() ([]string, error) {
	dir, _ := os.Getwd()
	files := []string{}
	// common layout
	common := filepath.Join(dir, "web", "html", "common", "page.html")
	if _, err := os.Stat(common); err == nil {
		files = append(files, common)
	}
	// components used
	theme := filepath.Join(dir, "web", "html", "component", "aThemeSwitch.html")
	if _, err := os.Stat(theme); err == nil {
		files = append(files, theme)
	}
	// page itself
	page := filepath.Join(dir, "web", "html", "subscription.html")
	if _, err := os.Stat(page); err == nil {
		files = append(files, page)
	} else {
		return nil, err
	}
	return files, nil
}

func (s *Server) Start() (err error) {
	// This is an anonymous function, no function name
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	subEnable, err := s.settingService.GetSubEnable()
	if err != nil {
		return err
	}
	if !subEnable {
		return nil
	}

	engine, err := s.initRouter()
	if err != nil {
		return err
	}

	certFile, err := s.settingService.GetSubCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.settingService.GetSubKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.settingService.GetSubListen()
	if err != nil {
		return err
	}
	port, err := s.settingService.GetSubPort()
	if err != nil {
		return err
	}

	listenAddr := net.JoinHostPort(listen, strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	if certFile != "" || keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err == nil {
			c := &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			listener = network.NewAutoHttpsListener(listener)
			listener = tls.NewListener(listener, c)
			logger.Info("Sub server running HTTPS on", listener.Addr())
		} else {
			logger.Error("Error loading certificates:", err)
			logger.Info("Sub server running HTTP on", listener.Addr())
		}
	} else {
		logger.Info("Sub server running HTTP on", listener.Addr())
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler: engine,
	}

	go func() {
		s.httpServer.Serve(listener)
	}()

	return nil
}

func (s *Server) Stop() error {
	s.cancel()

	var err1 error
	var err2 error
	if s.httpServer != nil {
		err1 = s.httpServer.Shutdown(s.ctx)
	}
	if s.listener != nil {
		err2 = s.listener.Close()
	}
	return common.Combine(err1, err2)
}

func (s *Server) GetCtx() context.Context {
	return s.ctx
}
