package main

import (
	"github.com/joho/godotenv"
	"os"
	"snwzt/rvc/cmd/sugar"
	"snwzt/rvc/internal/common"
)

func main() {
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

	sugar.Execute(os.Exit, os.Args[1:], redisConn, loggerInstance)
}
