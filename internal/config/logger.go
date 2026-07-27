package config

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitLogger() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	logDir := "logs"
	_ = os.MkdirAll(logDir, 0755)

	logFilePath := filepath.Join(logDir, "app.log")
	lf, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("failed to open log file: " + err.Error())
	}

	multiWriters := zerolog.MultiLevelWriter(os.Stdout, lf)

	log.Logger = zerolog.New(multiWriters).With().Timestamp().Logger()
	zerolog.DefaultContextLogger = &log.Logger
}
