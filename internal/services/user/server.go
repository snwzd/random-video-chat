package user

import (
	"errors"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/net/context"
	"net/http"
	"sync"
)

type Server struct {
	port          string
	engine        *echo.Echo
	httpHandlers  HttpServerHandler
	eventHandlers EventServerHandler
}

func NewServer(port string, engine *echo.Echo,
	httpHandlers HttpServerHandler, eventHandlers EventServerHandler) *Server {
	return &Server{
		port:          port,
		engine:        engine,
		httpHandlers:  httpHandlers,
		eventHandlers: eventHandlers,
	}
}

func (svc *Server) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	// user
	wg.Add(1)
	go func() {
		defer wg.Done()

		svc.engine.Use(echoprometheus.NewMiddleware("user_app"))
		svc.engine.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{http.MethodGet, http.MethodPost},
		}))

		svc.engine.GET("/health", svc.httpHandlers.checkHealth)
		svc.engine.GET("/metrics", echoprometheus.NewHandler())
		svc.engine.GET("/", svc.httpHandlers.home)
		svc.engine.POST("/register", svc.httpHandlers.registerUser)
		svc.engine.GET("/connection/:id", svc.httpHandlers.connection)
		svc.engine.GET("/match", svc.httpHandlers.matchUser)

		if err := svc.engine.Start(svc.port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// event
	wg.Add(1)
	go func() {
		defer wg.Done()

		svc.eventHandlers.Match(ctx)
	}()

	return <-errChan
}
