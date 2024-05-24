package main

import (
	"context"
	"github.com/joho/godotenv"
	"os"
	"os/signal"
	"rvc/internal/common"
	"rvc/internal/services/session"
	"sync"
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

	goroutines := make(map[string]context.CancelFunc)

	// metrics
	promMetrics := session.NewPromMetrics()
	go promMetrics.Counter(goroutines)

	// http
	storage := &session.Storage{
		RedisClient: redisConn,
	}

	handle := &session.ServerHandle{
		Store:      storage,
		Logger:     loggerInstance,
		Goroutines: goroutines,
	}

	server := session.NewServer(":"+os.Getenv("SESSION_SERVICE_PORT"), handle)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		loggerInstance.Info().Msg("starting session server")
		server.Run(ctx)
	}()

	wg.Wait()
}
