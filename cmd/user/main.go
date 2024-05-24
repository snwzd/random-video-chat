package main

import (
	"context"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"os"
	"os/signal"
	"rvc/internal/common"
	"rvc/internal/services/user"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	loggerInstance := common.NewLogger()

	if os.Getenv("SKIP_DOTENV") != "1" {
		if err := godotenv.Load("config/.env"); err != nil {
			loggerInstance.Err(err).Msg("unable to load env file")
			os.Exit(1)
		}
	}

	redisConn, err := common.NewRedisStore(os.Getenv("REDIS_URI"))
	if err != nil {
		loggerInstance.Err(err).Msg("failed to connect to redis")
		os.Exit(1)
	}

	serverInstance := echo.New()
	serverInstance.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:    true,
		LogStatus: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			loggerInstance.Info().
				Str("URI", v.URI).
				Int("status", v.Status).
				Msg("request")

			return nil
		},
	}))
	serverInstance.Use(middleware.Recover())

	serverInstance.Renderer, err = common.NewTemplate("web/*.html")
	if err != nil {
		loggerInstance.Err(err).Msg("unable to load templates")
	}

	httpStorage := &user.HttpStorage{
		RedisClient: redisConn,
	}

	httpHandle := &user.HttpServerHandle{
		SessionStore: sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY"))),
		Logger:       loggerInstance,
		Ctx:          ctx,
		Store:        httpStorage,
	}

	eventStorage := &user.EventStorage{
		RedisClient: redisConn,
	}

	eventHandle := &user.EventServerHandle{
		Logger: loggerInstance,
		Store:  eventStorage,
	}

	server := user.NewServer(":"+os.Getenv("USER_SERVICE_PORT"), serverInstance, httpHandle, eventHandle)

	go func() {
		if err := server.Run(ctx); err != nil {
			loggerInstance.Err(err).Msg("failed to start the server")
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := serverInstance.Shutdown(ctx); err != nil {
		loggerInstance.Err(err).Msg("failed to gracefully shutdown the server")
		os.Exit(1)
	}
}
