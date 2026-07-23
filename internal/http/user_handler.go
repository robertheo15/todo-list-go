package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"todo-list-go/internal/models"
	"todo-list-go/internal/repository"
	"todo-list-go/internal/service"
	pkgHttp "todo-list-go/pkg/http"
)

func (s *Server) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		pkgHttp.JSONError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	resp, err := s.service.Register(r.Context(), req)
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			pkgHttp.JSONError(w, http.StatusConflict, err.Error())
			return
		}
		pkgHttp.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	pkgHttp.JSONResponse(w, http.StatusCreated, resp)
}

func (s *Server) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		pkgHttp.JSONError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	resp, err := s.service.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			pkgHttp.JSONError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}
		pkgHttp.JSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	pkgHttp.JSONResponse(w, http.StatusOK, resp)
}
