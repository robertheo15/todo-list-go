package service

import (
	"context"
	"errors"

	"todo-list-go/internal/models"
	"todo-list-go/internal/repository"
)

func (s *Service) CreateTodo(ctx context.Context, userID int64, req models.CreateTodoRequest) (*models.Todo, error) {
	if req.Title == "" {
		return nil, errors.New("title is required")
	}

	todo := &models.Todo{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
	}

	if err := s.repo.CreateTodo(ctx, todo); err != nil {
		return nil, err
	}

	return todo, nil
}

func (s *Service) UpdateTodo(ctx context.Context, userID, todoID int64, req models.UpdateTodoRequest) (*models.Todo, error) {
	if req.Title == "" {
		return nil, errors.New("title is required")
	}

	existing, err := s.repo.GetTodoByID(ctx, todoID)
	if err != nil {
		if errors.Is(err, repository.ErrTodoNotFound) {
			return nil, repository.ErrTodoNotFound
		}
		return nil, err
	}

	if existing.UserID != userID {
		return nil, ErrForbidden
	}

	existing.Title = req.Title
	existing.Description = req.Description

	if err := s.repo.UpdateTodo(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

func (s *Service) DeleteTodo(ctx context.Context, userID, todoID int64) error {
	existing, err := s.repo.GetTodoByID(ctx, todoID)
	if err != nil {
		if errors.Is(err, repository.ErrTodoNotFound) {
			return repository.ErrTodoNotFound
		}
		return err
	}

	if existing.UserID != userID {
		return ErrForbidden
	}

	return s.repo.DeleteTodo(ctx, todoID)
}

func (s *Service) ListTodos(ctx context.Context, userID int64, page, limit int, search, sortBy, order string) (*models.TodoListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	items, total, err := s.repo.ListTodos(ctx, userID, page, limit, search, sortBy, order)
	if err != nil {
		return nil, err
	}

	return &models.TodoListResponse{
		Data:  items,
		Page:  page,
		Limit: limit,
		Total: total,
	}, nil
}
