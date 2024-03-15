package sugar

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"snwzt/rvc/internal/services/userconnection"
	"snwzt/rvc/internal/services/userconnection/handlers"
	"time"
)

type UserConnectionCmd struct {
	cmd *cobra.Command
}

func newUserConnectionCmd(redisConn *redis.Client, loggerInstance *zerolog.Logger) *UserConnectionCmd {
	root := &UserConnectionCmd{}
	cmd := &cobra.Command{
		Use:           "user-connection-service",
		Short:         "Run user connection service",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

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

			handle := &handlers.ServerHandle{
				Redis:  redisConn,
				Logger: loggerInstance,
				Ctx:    ctx,
			}

			server := userconnection.NewServer(":"+os.Getenv("USERCONN_SERVICE_PORT"), serverInstance, handle)

			go func() {
				if err := server.Run(); err != nil {
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

			return nil
		},
	}

	root.cmd = cmd
	return root
}
