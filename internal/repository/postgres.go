package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"todo-list-go/internal/models"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	repo := &PostgresRepository{db: db}
	//if err := repo.initTables(); err != nil {
	//	return nil, fmt.Errorf("failed to init tables: %w", err)
	//}
	return repo, nil
}

func (r *PostgresRepository) initTables() error {
	log.Info().Msg("Initializing database tables...")

	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`

	todosTable := `
	CREATE TABLE IF NOT EXISTS todos (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	resUsers, err := r.db.Exec(usersTable)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize users table")
		return err
	}
	rowsAffectedUsers, _ := resUsers.RowsAffected()
	log.Info().Int64("rows_affected", rowsAffectedUsers).Msg("Users table initialized or already exists")

	resTodos, err := r.db.Exec(todosTable)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize todos table")
		return err
	}
	rowsAffectedTodos, _ := resTodos.RowsAffected()
	log.Info().Int64("rows_affected", rowsAffectedTodos).Msg("Todos table initialized or already exists")

	log.Info().Msg("Database tables initialized successfully")
	return nil
}

func (r *PostgresRepository) CreateUser(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (name, email, password, created_at) VALUES ($1, $2, $3, $4) RETURNING id`
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query, user.Name, user.Email, user.Password, now).Scan(&user.ID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") || strings.Contains(err.Error(), "users_email_key") {
			return ErrUserAlreadyExists
		}
		return err
	}
	user.CreatedAt = now
	return nil
}

func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, name, email, password, created_at FROM users WHERE email = $1`
	row := r.db.QueryRowContext(ctx, query, email)
	var u models.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	query := `SELECT id, name, email, password, created_at FROM users WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	var u models.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *PostgresRepository) CreateTodo(ctx context.Context, todo *models.Todo) error {
	query := `INSERT INTO todos (user_id, title, description, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query, todo.UserID, todo.Title, todo.Description, now, now).Scan(&todo.ID)
	if err != nil {
		return err
	}
	todo.CreatedAt = now
	todo.UpdatedAt = now
	return nil
}

func (r *PostgresRepository) GetTodoByID(ctx context.Context, id int64) (*models.Todo, error) {
	query := `SELECT id, user_id, title, description, created_at, updated_at FROM todos WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	var t models.Todo
	err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTodoNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *PostgresRepository) UpdateTodo(ctx context.Context, todo *models.Todo) error {
	query := `UPDATE todos SET title = $1, description = $2, updated_at = $3 WHERE id = $4 AND user_id = $5`
	now := time.Now()
	res, err := r.db.ExecContext(ctx, query, todo.Title, todo.Description, now, todo.ID, todo.UserID)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrTodoNotFound
	}
	todo.UpdatedAt = now
	return nil
}

func (r *PostgresRepository) DeleteTodo(ctx context.Context, id int64) error {
	query := `DELETE FROM todos WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrTodoNotFound
	}
	return nil
}

func (r *PostgresRepository) ListTodos(ctx context.Context, userID int64, page, limit int, search, sortBy, order string) ([]models.Todo, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	var whereClause strings.Builder
	whereClause.WriteString("WHERE user_id = $1")
	args := []interface{}{userID}
	paramIdx := 2

	if search != "" {
		whereClause.WriteString(fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d)", paramIdx, paramIdx+1))
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
		paramIdx += 2
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM todos %s", whereClause.String())
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	validSortFields := map[string]string{
		"id":         "id",
		"title":      "title",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
	sortCol, ok := validSortFields[strings.ToLower(sortBy)]
	if !ok {
		sortCol = "id"
	}

	sortOrder := "ASC"
	if strings.ToUpper(order) == "DESC" {
		sortOrder = "DESC"
	}

	query := fmt.Sprintf("SELECT id, user_id, title, description, created_at, updated_at FROM todos %s ORDER BY %s %s LIMIT $%d OFFSET $%d",
		whereClause.String(), sortCol, sortOrder, paramIdx, paramIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	todos := make([]models.Todo, 0)
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		todos = append(todos, t)
	}

	return todos, total, nil
}
