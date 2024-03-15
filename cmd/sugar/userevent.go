package sugar

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"snwzt/rvc/internal/services/userevent"
	"snwzt/rvc/internal/services/userevent/handlers"
	"snwzt/rvc/internal/services/userevent/store"
	"sync"
)

type UserEventCmd struct {
	cmd *cobra.Command
}

func newUserEventCmd(redisConn *redis.Client, loggerInstance *zerolog.Logger) *UserEventCmd {
	root := &UserEventCmd{}
	cmd := &cobra.Command{
		Use:           "user-event-service",
		Short:         "Run user event service",
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
				Store:  storage,
				Logger: loggerInstance,
			}

			server := userevent.NewServer(handle)

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()

				server.Run(ctx)
			}()

			wg.Wait()

			return nil
		},
	}

	root.cmd = cmd
	return root
}
