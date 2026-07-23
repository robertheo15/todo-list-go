package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"todo-list-go/internal/models"
	"todo-list-go/internal/repository"
	"todo-list-go/internal/service"
	pkgHttp "todo-list-go/pkg/http"
)

func (s *Server) CreateTodoHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok {
		pkgHttp.JSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.CreateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		pkgHttp.JSONError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	todo, err := s.service.CreateTodo(r.Context(), userID, req)
	if err != nil {
		pkgHttp.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	pkgHttp.JSONResponse(w, http.StatusCreated, todo)
}

func parseListQueryParams(r *http.Request) (page, limit int, search, sortBy, order string) {
	query := r.URL.Query()
	page, _ = strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ = strconv.Atoi(query.Get("limit"))
	if limit < 1 {
		limit = 10
	}

	search = strings.TrimSpace(query.Get("search"))
	sortBy = strings.TrimSpace(query.Get("sort_by"))
	order = strings.TrimSpace(query.Get("order"))
	return
}

func (s *Server) GetTodosHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok {
		pkgHttp.JSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	page, limit, search, sortBy, order := parseListQueryParams(r)

	res, err := s.service.ListTodos(r.Context(), userID, page, limit, search, sortBy, order)
	if err != nil {
		pkgHttp.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pkgHttp.JSONResponse(w, http.StatusOK, res)
}

func (s *Server) UpdateTodoHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok {
		pkgHttp.JSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := r.PathValue("id")
	todoID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		pkgHttp.JSONError(w, http.StatusBadRequest, "Invalid todo ID")
		return
	}

	var req models.UpdateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		pkgHttp.JSONError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	todo, err := s.service.UpdateTodo(r.Context(), userID, todoID, req)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			pkgHttp.JSONError(w, http.StatusForbidden, "Forbidden")
			return
		}
		if errors.Is(err, repository.ErrTodoNotFound) {
			pkgHttp.JSONError(w, http.StatusNotFound, "Todo not found")
			return
		}
		pkgHttp.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	pkgHttp.JSONResponse(w, http.StatusOK, todo)
}

func (s *Server) DeleteTodoHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	if !ok {
		pkgHttp.JSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := r.PathValue("id")
	todoID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		pkgHttp.JSONError(w, http.StatusBadRequest, "Invalid todo ID")
		return
	}

	err = s.service.DeleteTodo(r.Context(), userID, todoID)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			pkgHttp.JSONError(w, http.StatusForbidden, "Forbidden")
			return
		}
		if errors.Is(err, repository.ErrTodoNotFound) {
			pkgHttp.JSONError(w, http.StatusNotFound, "Todo not found")
			return
		}
		pkgHttp.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
