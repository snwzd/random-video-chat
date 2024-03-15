package sugar

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"snwzt/rvc/internal/services/forwarder"
	"snwzt/rvc/internal/services/forwarder/handlers"
	"snwzt/rvc/internal/services/forwarder/store"
	"sync"
)

type ForwarderCmd struct {
	cmd *cobra.Command
}

func newForwarderCmd(redisConn *redis.Client, loggerInstance *zerolog.Logger) *ForwarderCmd {
	root := &ForwarderCmd{}
	cmd := &cobra.Command{
		Use:           "forwarder-service",
		Short:         "Run forwarder service",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			storage := &store.Storage{
				Redis: redisConn,
			}

			handle := &handlers.ServerHandle{
				Store:           storage,
				Logger:          loggerInstance,
				CancelForwarder: make(chan string),
			}

			server := forwarder.NewServer(handle)

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()

				loggerInstance.Info().Msg("starting forwarder server")
				server.Run(ctx)
			}()

			wg.Wait()
			return nil
		},
	}

	root.cmd = cmd
	return root
}
