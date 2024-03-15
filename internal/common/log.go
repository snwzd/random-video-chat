package common

import (
	"github.com/rs/zerolog"
	"os"
)

func NewLogger() *zerolog.Logger {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	return &logger
}
