package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"

	"todo-list-go/internal/models"
	"todo-list-go/internal/repository"
	"todo-list-go/internal/service"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	return db
}

func TestService_UserAuth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	svc := service.NewService(repo, "test-secret")
	ctx := context.Background()

	t.Run("Register success", func(t *testing.T) {
		req := models.RegisterRequest{
			Name:     "Test User",
			Email:    "test@example.com",
			Password: "secretpassword",
		}

		res, err := svc.Register(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Token == "" {
			t.Errorf("expected token to be non-empty")
		}
	})

	t.Run("Register duplicate email fails", func(t *testing.T) {
		req := models.RegisterRequest{
			Name:     "Test User 2",
			Email:    "test@example.com",
			Password: "secretpassword",
		}

		_, err := svc.Register(ctx, req)
		if !errors.Is(err, repository.ErrUserAlreadyExists) {
			t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
		}
	})

	t.Run("Login success", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "test@example.com",
			Password: "secretpassword",
		}

		res, err := svc.Login(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Token == "" {
			t.Errorf("expected token to be non-empty")
		}

		claims, err := svc.ValidateToken(res.Token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}
		if claims.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", claims.Email)
		}
	})

	t.Run("Login wrong password", func(t *testing.T) {
		req := models.LoginRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}

		_, err := svc.Login(ctx, req)
		if !errors.Is(err, service.ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestService_TodoCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	svc := service.NewService(repo, "test-secret")
	ctx := context.Background()

	// Register user 1
	u1, err := svc.Register(ctx, models.RegisterRequest{
		Name:     "User 1",
		Email:    "u1@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("failed to register u1: %v", err)
	}
	c1, _ := svc.ValidateToken(u1.Token)

	// Register user 2
	u2, err := svc.Register(ctx, models.RegisterRequest{
		Name:     "User 2",
		Email:    "u2@example.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("failed to register u2: %v", err)
	}
	c2, _ := svc.ValidateToken(u2.Token)

	var createdTodo *models.Todo

	t.Run("Create todo", func(t *testing.T) {
		todo, err := svc.CreateTodo(ctx, c1.UserID, models.CreateTodoRequest{
			Title:       "Buy milk",
			Description: "Whole milk",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if todo.ID == 0 {
			t.Errorf("expected non-zero ID")
		}
		createdTodo = todo
	})

	t.Run("Update todo owned by user", func(t *testing.T) {
		updated, err := svc.UpdateTodo(ctx, c1.UserID, createdTodo.ID, models.UpdateTodoRequest{
			Title:       "Buy milk and eggs",
			Description: "Whole milk and 12 eggs",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Title != "Buy milk and eggs" {
			t.Errorf("expected updated title, got %s", updated.Title)
		}
	})

	t.Run("Update todo owned by another user fails with ErrForbidden", func(t *testing.T) {
		_, err := svc.UpdateTodo(ctx, c2.UserID, createdTodo.ID, models.UpdateTodoRequest{
			Title:       "Hacked title",
			Description: "Hacked desc",
		})
		if !errors.Is(err, service.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("Delete todo owned by another user fails with ErrForbidden", func(t *testing.T) {
		err := svc.DeleteTodo(ctx, c2.UserID, createdTodo.ID)
		if !errors.Is(err, service.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("List todos with pagination", func(t *testing.T) {
		res, err := svc.ListTodos(ctx, c1.UserID, 1, 10, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Total != 1 {
			t.Errorf("expected total 1, got %d", res.Total)
		}
		if len(res.Data) != 1 {
			t.Errorf("expected 1 item, got %d", len(res.Data))
		}
	})

	t.Run("Delete todo owned by user success", func(t *testing.T) {
		err := svc.DeleteTodo(ctx, c1.UserID, createdTodo.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
