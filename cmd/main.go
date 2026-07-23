package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	internalHttp "todo-list-go/internal/http"
	"todo-list-go/internal/repository"
	"todo-list-go/internal/service"
)

func main() {
	dbDriver := os.Getenv("DB_DRIVER")
	dbCon := ""
	if dbDriver == "" {
		dbDriver = "sqlite"
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
		log.Fatalf("Failed to open database (%s): %v", dbDriver, err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database (%s): %v", dbDriver, err)
	}

	var repo repository.Repository
	switch dbDriver {
	case "postgres":
		repo, err = repository.NewPostgresRepository(db)
		if err != nil {
			log.Fatalf("Failed to initialize postgres repository: %v", err)
		}
	case "sqlite":
		repo, err = repository.NewSQLiteRepository(db)
		if err != nil {
			log.Fatalf("Failed to initialize sqlite repository: %v", err)
		}
	default:
		log.Fatalf("Unsupported DB_DRIVER: %s", dbDriver)
	}

	svc := service.NewService(repo, jwtSecret)
	server := internalHttp.NewServer(svc)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting To-Do API server on %s using %s database ...", addr, dbDriver)
	if err := http.ListenAndServe(addr, server.Router()); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
