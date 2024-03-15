package userconnection

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"snwzt/rvc/internal/services/userconnection/handlers"
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
	svc.engine.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	svc.engine.GET("/health", svc.handlers.CheckHealth)
	svc.engine.GET("/connection/:id", svc.handlers.Connection)

	if err := svc.engine.Start(svc.port); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
