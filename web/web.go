package web

import (
	"context"
	"crypto/tls"
	"embed"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"x-ui/config"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/controller"
	"x-ui/web/job"
	"x-ui/web/network"
	"x-ui/web/service"

	"github.com/BurntSushi/toml"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/robfig/cron/v3"
	"golang.org/x/text/language"
)

//go:embed assets/*
var assetsFS embed.FS

//go:embed html/*
var htmlFS embed.FS

//go:embed translation/*
var i18nFS embed.FS

var startTime = time.Now()

type wrapAssetsFS struct {
	embed.FS
}

func (f *wrapAssetsFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open("assets/" + name)
	if err != nil {
		return nil, err
	}
	return &wrapAssetsFile{
		File: file,
	}, nil
}

type wrapAssetsFile struct {
	fs.File
}

func (f *wrapAssetsFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &wrapAssetsFileInfo{
		FileInfo: info,
	}, nil
}

type wrapAssetsFileInfo struct {
	fs.FileInfo
}

func (f *wrapAssetsFileInfo) ModTime() time.Time {
	return startTime
}

type Server struct {
	httpServer *http.Server
	listener   net.Listener

	index  *controller.IndexController
	server *controller.ServerController
	xui    *controller.XUIController
	api    *controller.APIController

	xrayService    service.XrayService
	settingService service.SettingService
	inboundService service.InboundService

	cron *cron.Cron

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

func (s *Server) getHtmlFiles() ([]string, error) {
	files := make([]string, 0)
	dir, _ := os.Getwd()
	err := fs.WalkDir(os.DirFS(dir), "web/html", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *Server) getHtmlTemplate(funcMap template.FuncMap) (*template.Template, error) {
	t := template.New("").Funcs(funcMap)
	err := fs.WalkDir(htmlFS, "html", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			newT, err := t.ParseFS(htmlFS, path+"/*.html")
			if err != nil {
				// ignore
				return nil
			}
			t = newT
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Server) initRouter() (*gin.Engine, error) {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	secret, err := s.settingService.GetSecret()
	if err != nil {
		return nil, err
	}

	basePath, err := s.settingService.GetBasePath()
	if err != nil {
		return nil, err
	}
	assetsBasePath := basePath + "assets/"

	store := cookie.NewStore(secret)
	engine.Use(sessions.Sessions("session", store))
	engine.Use(func(c *gin.Context) {
		c.Set("base_path", basePath)
	})
	engine.Use(func(c *gin.Context) {
		uri := c.Request.RequestURI
		if strings.HasPrefix(uri, assetsBasePath) {
			c.Header("Cache-Control", "max-age=31536000")
		}
	})
	err = s.initI18n(engine)
	if err != nil {
		return nil, err
	}

	if config.IsDebug() {
		// for develop
		files, err := s.getHtmlFiles()
		if err != nil {
			return nil, err
		}
		engine.LoadHTMLFiles(files...)
		engine.StaticFS(basePath+"assets", http.FS(os.DirFS("web/assets")))
	} else {
		// for prod
		t, err := s.getHtmlTemplate(engine.FuncMap)
		if err != nil {
			return nil, err
		}
		engine.SetHTMLTemplate(t)
		engine.StaticFS(basePath+"assets", http.FS(&wrapAssetsFS{FS: assetsFS}))
	}

	g := engine.Group(basePath)

	s.index = controller.NewIndexController(g)
	s.server = controller.NewServerController(g)
	s.xui = controller.NewXUIController(g)
	s.api = controller.NewAPIController(g)

	return engine, nil
}

func (s *Server) initI18n(engine *gin.Engine) error {
	bundle := i18n.NewBundle(language.SimplifiedChinese)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	err := fs.WalkDir(i18nFS, "translation", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := i18nFS.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = bundle.ParseMessageFileBytes(data, path)
		return err
	})
	if err != nil {
		return err
	}

	findI18nParamNames := func(key string) []string {
		names := make([]string, 0)
		keyLen := len(key)
		for i := 0; i < keyLen-1; i++ {
			if key[i:i+2] == "{{" { // 判断开头 "{{"
				j := i + 2
				isFind := false
				for ; j < keyLen-1; j++ {
					if key[j:j+2] == "}}" { // 结尾 "}}"
						isFind = true
						break
					}
				}
				if isFind {
					names = append(names, key[i+3:j])
				}
			}
		}
		return names
	}

	var localizer *i18n.Localizer

	I18n := func(key string, params ...string) (string, error) {
		names := findI18nParamNames(key)
		if len(names) != len(params) {
			return "", common.NewError("find names:", names, "---------- params:", params, "---------- num not equal")
		}
		templateData := map[string]interface{}{}
		for i := range names {
			templateData[names[i]] = params[i]
		}
		return localizer.Localize(&i18n.LocalizeConfig{
			MessageID:    key,
			TemplateData: templateData,
		})
	}

	engine.FuncMap["i18n"]  = I18n;

	engine.Use(func(c *gin.Context) {
		//accept := c.GetHeader("Accept-Language")

		var lang string

		if cookie, err := c.Request.Cookie("lang"); err == nil {
			lang = cookie.Value
		} else {
			lang = c.GetHeader("Accept-Language")
		}

		localizer = i18n.NewLocalizer(bundle, lang)
		c.Set("localizer", localizer)
		c.Set("I18n" , I18n)
		c.Next()
	})

	return nil
}

func (s *Server) startTask() {
	err := s.xrayService.RestartXray(true)
	if err != nil {
		logger.Warning("start xray failed:", err)
	}
	// 每 30 秒检查一次 xray 是否在运行
	s.cron.AddJob("@every 30s", job.NewCheckXrayRunningJob())

	go func() {
		time.Sleep(time.Second * 5)
		// 每 10 秒统计一次流量，首次启动延迟 5 秒，与重启 xray 的时间错开
		s.cron.AddJob("@every 10s", job.NewXrayTrafficJob())
	}()

	// 每 30 秒检查一次 inbound 流量超出和到期的情况
	s.cron.AddJob("@every 30s", job.NewCheckInboundJob())

	// 每一天提示一次流量情况,上海时间8点30
	var entry cron.EntryID
	isTgbotenabled, err := s.settingService.GetTgbotenabled()
	if (err == nil) && (isTgbotenabled) {
		runtime, err := s.settingService.GetTgbotRuntime()
		if err != nil || runtime == "" {
			logger.Errorf("Add NewStatsNotifyJob error[%s],Runtime[%s] invalid,wil run default", err, runtime)
			runtime = "@daily"
		}
		logger.Infof("Tg notify enabled,run at %s", runtime)
		entry, err = s.cron.AddJob(runtime, job.NewStatsNotifyJob())
		if err != nil {
			logger.Warning("Add NewStatsNotifyJob error", err)
			return
		}
		// listen for TG bot income messages
		go job.NewStatsNotifyJob().OnReceive()
	} else {
		s.cron.Remove(entry)
	}
}

func (s *Server) Start() (err error) {
	//这是一个匿名函数，没没有函数名
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	loc, err := s.settingService.GetTimeLocation()
	if err != nil {
		return err
	}
	s.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	s.cron.Start()

	engine, err := s.initRouter()
	if err != nil {
		return err
	}

	certFile, err := s.settingService.GetCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.settingService.GetKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.settingService.GetListen()
	if err != nil {
		return err
	}
	port, err := s.settingService.GetPort()
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
		if err != nil {
			listener.Close()
			return err
		}
		c := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		listener = network.NewAutoHttpsListener(listener)
		listener = tls.NewListener(listener, c)
	}

	if certFile != "" || keyFile != "" {
		logger.Info("web server run https on", listener.Addr())
	} else {
		logger.Info("web server run http on", listener.Addr())
	}
	s.listener = listener

	s.startTask()

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
	s.xrayService.StopXray()
	if s.cron != nil {
		s.cron.Stop()
	}
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

func (s *Server) GetCron() *cron.Cron {
	return s.cron
}
