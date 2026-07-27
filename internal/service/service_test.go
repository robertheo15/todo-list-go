package service_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"

	"todo-list-go/internal/models"
	"todo-list-go/internal/repository"
	"todo-list-go/internal/service"
)

func setupTestContext(t *testing.T) context.Context {
	t.Helper()
	testLogger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	return testLogger.WithContext(context.Background())
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	return db
}

func TestService_UserRegister_TableDriven(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	svc := service.NewService(repo, "test-secret")

	testCases := []struct {
		name        string
		req         models.RegisterRequest
		expectedErr error
	}{
		{
			name: "Success - Valid Request",
			req: models.RegisterRequest{
				Name:     "Alice",
				Email:    "alice@example.com",
				Password: "password123",
			},
			expectedErr: nil,
		},
		{
			name: "Validation Error - Missing Name",
			req: models.RegisterRequest{
				Name:     "",
				Email:    "bob@example.com",
				Password: "password123",
			},
			expectedErr: errors.New("name, email, and password are required"),
		},
		{
			name: "Validation Error - Missing Email",
			req: models.RegisterRequest{
				Name:     "Bob",
				Email:    "",
				Password: "password123",
			},
			expectedErr: errors.New("name, email, and password are required"),
		},
		{
			name: "Validation Error - Missing Password",
			req: models.RegisterRequest{
				Name:     "Bob",
				Email:    "bob@example.com",
				Password: "",
			},
			expectedErr: errors.New("name, email, and password are required"),
		},
		{
			name: "Duplicate Email - Already Exists",
			req: models.RegisterRequest{
				Name:     "Alice Clone",
				Email:    "alice@example.com",
				Password: "password123",
			},
			expectedErr: repository.ErrUserAlreadyExists,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupTestContext(t)
			res, err := svc.Register(ctx, tc.req)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tc.expectedErr)
				}
				if !errors.Is(err, tc.expectedErr) && err.Error() != tc.expectedErr.Error() {
					t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res == nil || res.Token == "" || res.RefreshToken == "" {
				t.Errorf("expected valid AuthResponse with non-empty tokens")
			}
		})
	}
}

func TestService_UserLogin_TableDriven(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	svc := service.NewService(repo, "test-secret")
	ctx := setupTestContext(t)

	// Pre-register user
	_, err = svc.Register(ctx, models.RegisterRequest{
		Name:     "Login User",
		Email:    "user@example.com",
		Password: "correctpassword",
	})
	if err != nil {
		t.Fatalf("failed to setup user: %v", err)
	}

	testCases := []struct {
		name        string
		req         models.LoginRequest
		expectedErr error
	}{
		{
			name: "Success - Valid Credentials",
			req: models.LoginRequest{
				Email:    "user@example.com",
				Password: "correctpassword",
			},
			expectedErr: nil,
		},
		{
			name: "Validation Error - Missing Email",
			req: models.LoginRequest{
				Email:    "",
				Password: "correctpassword",
			},
			expectedErr: errors.New("email and password are required"),
		},
		{
			name: "Validation Error - Missing Password",
			req: models.LoginRequest{
				Email:    "user@example.com",
				Password: "",
			},
			expectedErr: errors.New("email and password are required"),
		},
		{
			name: "Invalid Credentials - Non-existing Email",
			req: models.LoginRequest{
				Email:    "nonexisting@example.com",
				Password: "correctpassword",
			},
			expectedErr: service.ErrInvalidCredentials,
		},
		{
			name: "Invalid Credentials - Wrong Password",
			req: models.LoginRequest{
				Email:    "user@example.com",
				Password: "wrongpassword",
			},
			expectedErr: service.ErrInvalidCredentials,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testCtx := setupTestContext(t)
			res, err := svc.Login(testCtx, tc.req)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.expectedErr)
				}
				if !errors.Is(err, tc.expectedErr) && err.Error() != tc.expectedErr.Error() {
					t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res == nil || res.Token == "" {
				t.Errorf("expected valid AuthResponse")
			}
		})
	}
}

func TestService_TodoOperations_TableDriven(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	svc := service.NewService(repo, "test-secret")
	ctx := setupTestContext(t)

	// Setup users
	u1Res, _ := svc.Register(ctx, models.RegisterRequest{Name: "User 1", Email: "u1@test.com", Password: "pwd"})
	u2Res, _ := svc.Register(ctx, models.RegisterRequest{Name: "User 2", Email: "u2@test.com", Password: "pwd"})
	c1, _ := svc.ValidateToken(u1Res.Token)
	c2, _ := svc.ValidateToken(u2Res.Token)

	todo, err := svc.CreateTodo(ctx, c1.UserID, models.CreateTodoRequest{Title: "Initial Todo", Description: "Desc"})
	if err != nil {
		t.Fatalf("failed to create initial todo: %v", err)
	}

	t.Run("CreateTodo Validation", func(t *testing.T) {
		testCases := []struct {
			name        string
			req         models.CreateTodoRequest
			expectedErr bool
		}{
			{
				name:        "Valid Todo",
				req:         models.CreateTodoRequest{Title: "Valid Title", Description: "Desc"},
				expectedErr: false,
			},
			{
				name:        "Empty Title",
				req:         models.CreateTodoRequest{Title: "", Description: "Desc"},
				expectedErr: true,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				testCtx := setupTestContext(t)
				_, err := svc.CreateTodo(testCtx, c1.UserID, tc.req)
				if (err != nil) != tc.expectedErr {
					t.Fatalf("expected error status %v, got %v", tc.expectedErr, err)
				}
			})
		}
	})

	t.Run("UpdateTodo Permissions & Validation", func(t *testing.T) {
		testCases := []struct {
			name        string
			userID      int64
			todoID      int64
			req         models.UpdateTodoRequest
			expectedErr error
		}{
			{
				name:        "Update Success - Owner",
				userID:      c1.UserID,
				todoID:      todo.ID,
				req:         models.UpdateTodoRequest{Title: "Updated Title", Description: "Updated Desc"},
				expectedErr: nil,
			},
			{
				name:        "Empty Title Validation",
				userID:      c1.UserID,
				todoID:      todo.ID,
				req:         models.UpdateTodoRequest{Title: "", Description: "Desc"},
				expectedErr: errors.New("title is required"),
			},
			{
				name:        "Forbidden Update - Non Owner",
				userID:      c2.UserID,
				todoID:      todo.ID,
				req:         models.UpdateTodoRequest{Title: "Hacked", Description: "Hacked"},
				expectedErr: service.ErrForbidden,
			},
			{
				name:        "Not Found Todo",
				userID:      c1.UserID,
				todoID:      99999,
				req:         models.UpdateTodoRequest{Title: "Title", Description: "Desc"},
				expectedErr: repository.ErrTodoNotFound,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				testCtx := setupTestContext(t)
				_, err := svc.UpdateTodo(testCtx, tc.userID, tc.todoID, tc.req)
				if tc.expectedErr != nil {
					if !errors.Is(err, tc.expectedErr) && (err == nil || err.Error() != tc.expectedErr.Error()) {
						t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("DeleteTodo Permissions", func(t *testing.T) {
		testCases := []struct {
			name        string
			userID      int64
			todoID      int64
			expectedErr error
		}{
			{
				name:        "Forbidden Delete - Non Owner",
				userID:      c2.UserID,
				todoID:      todo.ID,
				expectedErr: service.ErrForbidden,
			},
			{
				name:        "Not Found Delete",
				userID:      c1.UserID,
				todoID:      99999,
				expectedErr: repository.ErrTodoNotFound,
			},
			{
				name:        "Success Delete - Owner",
				userID:      c1.UserID,
				todoID:      todo.ID,
				expectedErr: nil,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				testCtx := setupTestContext(t)
				err := svc.DeleteTodo(testCtx, tc.userID, tc.todoID)
				if tc.expectedErr != nil {
					if !errors.Is(err, tc.expectedErr) && (err == nil || err.Error() != tc.expectedErr.Error()) {
						t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})
}

// TestConcurrentOperations verifies race safety under concurrent requests
func TestConcurrentOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	svc := service.NewService(repo, "test-secret")
	ctx := setupTestContext(t)

	authRes, err := svc.Register(ctx, models.RegisterRequest{
		Name:     "Concurrent User",
		Email:    "concurrent@test.com",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("failed to register concurrent user: %v", err)
	}
	claims, _ := svc.ValidateToken(authRes.Token)

	const numWorkers = 10
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			workerCtx := setupTestContext(t)
			title := fmt.Sprintf("Todo Worker %d", workerID)
			_, err := svc.CreateTodo(workerCtx, claims.UserID, models.CreateTodoRequest{
				Title:       title,
				Description: "Concurrent creation",
			})
			if err != nil {
				t.Errorf("worker %d failed to create todo: %v", workerID, err)
			}
		}(i)
	}

	wg.Wait()

	res, err := svc.ListTodos(ctx, claims.UserID, 1, 100, "", "", "")
	if err != nil {
		t.Fatalf("failed to list todos after concurrent creation: %v", err)
	}

	if res.Total != numWorkers {
		t.Errorf("expected total %d todos, got %d", numWorkers, res.Total)
	}
}
