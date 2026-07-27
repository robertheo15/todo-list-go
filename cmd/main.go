package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"

	"todo-list-go/internal/config"
	internalHttp "todo-list-go/internal/http"
	"todo-list-go/internal/repository"
	"todo-list-go/internal/service"
)

func main() {
	// Initialize structured zerolog logging (stdout + file)
	config.InitLogger()
	config.LoadEnv()

	dbDriver := config.GetEnv("DB_DRIVER", "sqlite")
	dbCon := ""
	if dbDriver == "sqlite" {
		dbCon = os.Getenv("DB_PATH")
	} else if dbDriver == "postgres" {
		dbCon = os.Getenv("DB_CONN")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "super-secret-key-change-in-production"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := sql.Open(dbDriver, dbCon)
	if err != nil {
		log.Fatal().Err(err).Str("driver", dbDriver).Msg("Failed to open database")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database connection cleanly")
		} else {
			log.Info().Msg("Database connection closed cleanly")
		}
	}()

	if err := db.Ping(); err != nil {
		log.Fatal().Err(err).Str("driver", dbDriver).Msg("Failed to ping database")
	}

	var repo repository.Repository
	switch dbDriver {
	case "postgres":
		repo, err = repository.NewPostgresRepository(db)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize postgres repository")
		}
	case "sqlite":
		repo, err = repository.NewSQLiteRepository(db)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize sqlite repository")
		}
	default:
		log.Fatal().Str("driver", dbDriver).Msg("Unsupported DB_DRIVER")
	}

	svc := service.NewService(repo, jwtSecret)
	server := internalHttp.NewServer(svc)

	addr := fmt.Sprintf(":%s", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: server.Router(),
	}

	// Channel to listen for errors from the HTTP server
	serverErrors := make(chan error, 1)

	// Start HTTP server in a separate goroutine
	go func() {
		log.Info().Str("addr", addr).Str("driver", dbDriver).Msg("Starting To-Do API server")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// Channel to listen for OS signals (SIGINT, SIGTERM)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until a signal or server error is received
	select {
	case err := <-serverErrors:
		log.Fatal().Err(err).Msg("Server failed to start or died unexpectedly")
	case sig := <-shutdown:
		log.Info().Str("signal", sig.String()).Msg("Shutdown signal received, initiating graceful shutdown")

		// Create a deadline context for shutdown (e.g., 10 seconds)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Server forced to shutdown")
			if err := srv.Close(); err != nil {
				log.Error().Err(err).Msg("Error closing server")
			}
		} else {
			log.Info().Msg("HTTP server shut down gracefully")
		}
	}
}
