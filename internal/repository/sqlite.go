package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"todo-list-go/internal/models"

	_ "modernc.org/sqlite"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	repo := &SQLiteRepository{db: db}
	if err := repo.initTables(); err != nil {
		return nil, fmt.Errorf("failed to init tables: %w", err)
	}
	return repo, nil
}

func (r *SQLiteRepository) initTables() error {
	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	todosTable := `
	CREATE TABLE IF NOT EXISTS todos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);`

	if _, err := r.db.Exec(usersTable); err != nil {
		return err
	}
	if _, err := r.db.Exec(todosTable); err != nil {
		return err
	}

	return nil
}

func (r *SQLiteRepository) CreateUser(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (name, email, password, created_at) VALUES (?, ?, ?, ?)`
	now := time.Now()
	res, err := r.db.ExecContext(ctx, query, user.Name, user.Email, user.Password, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrUserAlreadyExists
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = id
	user.CreatedAt = now
	return nil
}

func (r *SQLiteRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, name, email, password, created_at FROM users WHERE email = ?`
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

func (r *SQLiteRepository) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	query := `SELECT id, name, email, password, created_at FROM users WHERE id = ?`
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

func (r *SQLiteRepository) CreateTodo(ctx context.Context, todo *models.Todo) error {
	query := `INSERT INTO todos (user_id, title, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	now := time.Now()
	res, err := r.db.ExecContext(ctx, query, todo.UserID, todo.Title, todo.Description, now, now)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	todo.ID = id
	todo.CreatedAt = now
	todo.UpdatedAt = now
	return nil
}

func (r *SQLiteRepository) GetTodoByID(ctx context.Context, id int64) (*models.Todo, error) {
	query := `SELECT id, user_id, title, description, created_at, updated_at FROM todos WHERE id = ?`
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

func (r *SQLiteRepository) UpdateTodo(ctx context.Context, todo *models.Todo) error {
	query := `UPDATE todos SET title = ?, description = ?, updated_at = ? WHERE id = ? AND user_id = ?`
	now := time.Now()
	res, err := r.db.ExecContext(ctx, query, todo.Title, todo.Description, now, todo.UserID, todo.ID)
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

func (r *SQLiteRepository) DeleteTodo(ctx context.Context, id int64) error {
	query := `DELETE FROM todos WHERE id = ?`
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

func (r *SQLiteRepository) ListTodos(ctx context.Context, userID int64, page, limit int, search, sortBy, order string) ([]models.Todo, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	var whereClause strings.Builder
	whereClause.WriteString("WHERE user_id = ?")
	args := []interface{}{userID}

	if search != "" {
		whereClause.WriteString(" AND (title LIKE ? OR description LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
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

	query := fmt.Sprintf("SELECT id, user_id, title, description, created_at, updated_at FROM todos %s ORDER BY %s %s LIMIT ? OFFSET ?",
		whereClause.String(), sortCol, sortOrder)

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
