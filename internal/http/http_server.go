package http

import (
	"net/http"

	"todo-list-go/internal/service"
)

type Server struct {
	service *service.Service
	mux     *http.ServeMux
}

func NewServer(svc *service.Service) *Server {
	s := &Server{
		service: svc,
		mux:     http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Public routes
	s.mux.HandleFunc("POST /register", s.RegisterHandler)
	s.mux.HandleFunc("POST /login", s.LoginHandler)

	// Protected routes
	s.mux.Handle("POST /todos", s.AuthMiddleware(http.HandlerFunc(s.CreateTodoHandler)))
	s.mux.Handle("GET /todos", s.AuthMiddleware(http.HandlerFunc(s.GetTodosHandler)))
	s.mux.Handle("PUT /todos/{id}", s.AuthMiddleware(http.HandlerFunc(s.UpdateTodoHandler)))
	s.mux.Handle("DELETE /todos/{id}", s.AuthMiddleware(http.HandlerFunc(s.DeleteTodoHandler)))
}

func (s *Server) Router() http.Handler {
	// Apply Rate Limit Middleware to all routes
	return s.RateLimitMiddleware(s.mux)
}
