package http_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"

	internalHttp "todo-list-go/internal/http"
	"todo-list-go/internal/repository"
	"todo-list-go/internal/service"
)

func setupTestServer(t *testing.T) (*internalHttp.Server, string) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	secret := "test-jwt-secret"
	svc := service.NewService(repo, secret)
	server := internalHttp.NewServer(svc)
	return server, secret
}

func TestHTTP_EndToEnd(t *testing.T) {
	server, _ := setupTestServer(t)
	handler := server.Router()

	var token string

	t.Run("POST /register", func(t *testing.T) {
		body := map[string]string{
			"name":     "Jane Doe",
			"email":    "jane@example.com",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		token = resp["token"]
		if token == "" {
			t.Fatalf("expected token in response")
		}
	})

	t.Run("POST /login", func(t *testing.T) {
		body := map[string]string{
			"email":    "jane@example.com",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rr.Code)
		}
	})

	t.Run("POST /todos without token -> 401 Unauthorized", func(t *testing.T) {
		body := map[string]string{
			"title":       "Buy groceries",
			"description": "Milk, Eggs, Bread",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/todos", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized, got %d", rr.Code)
		}
	})

	var todoID float64

	t.Run("POST /todos with token -> 201 Created", func(t *testing.T) {
		body := map[string]string{
			"title":       "Buy groceries",
			"description": "Milk, Eggs, Bread",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/todos", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		todoID = resp["id"].(float64)
		if resp["title"] != "Buy groceries" {
			t.Errorf("expected title 'Buy groceries', got %v", resp["title"])
		}
	})

	t.Run("GET /todos with token -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/todos?page=1&limit=10", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rr.Code)
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["total"].(float64) != 1 {
			t.Errorf("expected total 1, got %v", resp["total"])
		}
	})

	t.Run("PUT /todos/{id} with token -> 200 OK", func(t *testing.T) {
		body := map[string]string{
			"title":       "Buy groceries and snacks",
			"description": "Milk, Eggs, Bread, Chips",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", "/todos/1", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "1")
		_ = todoID
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("DELETE /todos/{id} with token -> 204 No Content", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/todos/1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.SetPathValue("id", "1")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected status 204 No Content, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})
}
