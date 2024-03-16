package main

import (
	"context"
	"github.com/joho/godotenv"
	"os"
	"os/signal"
	"snwzt/rvc/internal/common"
	"snwzt/rvc/internal/services/forwarder"
	"snwzt/rvc/internal/services/forwarder/handlers"
	"snwzt/rvc/internal/services/forwarder/store"
	"sync"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	loggerInstance := common.NewLogger()

	if err := godotenv.Load("config/.env"); err != nil {
		loggerInstance.Err(err).Msg("unable to load env file")
		os.Exit(1)
	}

	redisConn, err := common.NewRedisStore(os.Getenv("REDIS_URI"))
	if err != nil {
		loggerInstance.Err(err).Msg("failed to connect to redis")
		os.Exit(1)
	}

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
}
