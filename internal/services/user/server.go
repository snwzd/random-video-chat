package user

import (
	"errors"
	"github.com/labstack/echo-contrib/echoprometheus"
	"net/http"
	"snwzt/rvc/internal/services/user/handlers"

	"github.com/labstack/echo/v4"
)

type Server struct {
	port     string
	engine   *echo.Echo
	handlers handlers.ServerHandler
}

func NewServer(port string, engine *echo.Echo, handlers handlers.ServerHandler) *Server {
	return &Server{
		port:     port,
		engine:   engine,
		handlers: handlers,
	}
}

func (svc *Server) Run() error {
	svc.engine.GET("/health", svc.handlers.CheckHealth)
	svc.engine.GET("/metrics", echoprometheus.NewHandler())
	
	svc.engine.GET("/", svc.handlers.Home)
	svc.engine.POST("/register", svc.handlers.RegisterUser)
	svc.engine.GET("/match", svc.handlers.MatchUser)

	if err := svc.engine.Start(svc.port); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
