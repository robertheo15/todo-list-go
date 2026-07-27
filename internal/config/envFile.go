package config

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		// Log a debug message, as the .env file might not exist in production/docker environments
		log.Debug().Msg("no .env file found, relying on environment variables")
		return
	}
	log.Info().Msg(".env file loaded successfully")
}

func GetEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
