package sub

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strconv"
	"x-ui/config"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/middleware"
	"x-ui/web/network"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

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
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	subPath, err := s.settingService.GetSubPath()
	if err != nil {
		return nil, err
	}

	subDomain, err := s.settingService.GetSubDomain()
	if err != nil {
		return nil, err
	}

	if subDomain != "" {
		engine.Use(middleware.DomainValidatorMiddleware(subDomain))
	}

	g := engine.Group(subPath)

	s.sub = NewSUBController(g)

	return engine, nil
}

func (s *Server) Start() (err error) {
	//This is an anonymous function, no function name
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
		logger.Info("Sub server run https on", listener.Addr())
	} else {
		logger.Info("Sub server run http on", listener.Addr())
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
