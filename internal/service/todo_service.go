package service

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"todo-list-go/internal/models"
	"todo-list-go/internal/repository"
)

func (s *Service) CreateTodo(ctx context.Context, userID int64, req models.CreateTodoRequest) (*models.Todo, error) {
	logger := log.Ctx(ctx).With().Str("func", "CreateTodo").Logger()
	logger.Debug().Int64("user_id", userID).Str("title", req.Title).Msg("creating todo")

	if req.Title == "" {
		logger.Warn().Int64("user_id", userID).Msg("todo title is required")
		return nil, errors.New("title is required")
	}

	todo := &models.Todo{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
	}

	if err := s.repo.CreateTodo(ctx, todo); err != nil {
		logger.Error().Err(err).Int64("user_id", userID).Msg("failed to create todo in repo")
		return nil, err
	}

	logger.Info().Int64("user_id", userID).Int64("todo_id", todo.ID).Msg("todo created successfully")
	return todo, nil
}

func (s *Service) UpdateTodo(ctx context.Context, userID, todoID int64, req models.UpdateTodoRequest) (*models.Todo, error) {
	logger := log.Ctx(ctx).With().Str("func", "UpdateTodo").Logger()
	logger.Debug().Int64("user_id", userID).Int64("todo_id", todoID).Msg("updating todo")

	if req.Title == "" {
		logger.Warn().Int64("user_id", userID).Int64("todo_id", todoID).Msg("todo title is required")
		return nil, errors.New("title is required")
	}

	existing, err := s.repo.GetTodoByID(ctx, todoID)
	if err != nil {
		if errors.Is(err, repository.ErrTodoNotFound) {
			logger.Warn().Int64("todo_id", todoID).Msg("todo not found")
			return nil, repository.ErrTodoNotFound
		}
		logger.Error().Err(err).Int64("todo_id", todoID).Msg("failed to get todo by id")
		return nil, err
	}

	if existing.UserID != userID {
		logger.Warn().Int64("user_id", userID).Int64("existing_user_id", existing.UserID).Msg("forbidden todo update attempt")
		return nil, ErrForbidden
	}

	existing.Title = req.Title
	existing.Description = req.Description

	if err := s.repo.UpdateTodo(ctx, existing); err != nil {
		logger.Error().Err(err).Int64("todo_id", todoID).Msg("failed to update todo in repo")
		return nil, err
	}

	logger.Info().Int64("user_id", userID).Int64("todo_id", todoID).Msg("todo updated successfully")
	return existing, nil
}

func (s *Service) DeleteTodo(ctx context.Context, userID, todoID int64) error {
	logger := log.Ctx(ctx).With().Str("func", "DeleteTodo").Logger()
	logger.Debug().Int64("user_id", userID).Int64("todo_id", todoID).Msg("deleting todo")

	existing, err := s.repo.GetTodoByID(ctx, todoID)
	if err != nil {
		if errors.Is(err, repository.ErrTodoNotFound) {
			logger.Warn().Int64("todo_id", todoID).Msg("todo not found")
			return repository.ErrTodoNotFound
		}
		logger.Error().Err(err).Int64("todo_id", todoID).Msg("failed to get todo by id")
		return err
	}

	if existing.UserID != userID {
		logger.Warn().Int64("user_id", userID).Int64("existing_user_id", existing.UserID).Msg("forbidden todo delete attempt")
		return ErrForbidden
	}

	if err := s.repo.DeleteTodo(ctx, todoID); err != nil {
		logger.Error().Err(err).Int64("todo_id", todoID).Msg("failed to delete todo in repo")
		return err
	}

	logger.Info().Int64("user_id", userID).Int64("todo_id", todoID).Msg("todo deleted successfully")
	return nil
}

func (s *Service) ListTodos(ctx context.Context, userID int64, page, limit int, search, sortBy, order string) (*models.TodoListResponse, error) {
	logger := log.Ctx(ctx).With().Str("func", "ListTodos").Logger()
	logger.Debug().Int64("user_id", userID).Int("page", page).Int("limit", limit).Str("search", search).Msg("listing todos")

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	items, total, err := s.repo.ListTodos(ctx, userID, page, limit, search, sortBy, order)
	if err != nil {
		logger.Error().Err(err).Int64("user_id", userID).Msg("failed to list todos from repo")
		return nil, err
	}

	logger.Info().Int64("user_id", userID).Int("count", len(items)).Int64("total", int64(total)).Msg("todos listed successfully")
	return &models.TodoListResponse{
		Data:  items,
		Page:  page,
		Limit: limit,
		Total: total,
	}, nil
}
