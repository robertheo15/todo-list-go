package repository

import (
	"context"
	"errors"

	"todo-list-go/internal/models"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user with this email already exists")
	ErrTodoNotFound      = errors.New("todo item not found")
)

type Repository interface {
	// User operations
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id int64) (*models.User, error)

	// Todo operations
	CreateTodo(ctx context.Context, todo *models.Todo) error
	GetTodoByID(ctx context.Context, id int64) (*models.Todo, error)
	UpdateTodo(ctx context.Context, todo *models.Todo) error
	DeleteTodo(ctx context.Context, id int64) error
	ListTodos(ctx context.Context, userID int64, page, limit int, search, sortBy, order string) ([]models.Todo, int, error)
}
