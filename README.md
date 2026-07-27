# Todo List Go API

A production-grade RESTful API built in **Go 1.25** for managing users and todo items, supporting both **SQLite** and **PostgreSQL** storage, JWT authentication, rate limiting, and correlation-traced structured logging.

---

## 🚀 Features

- **Authentication & Security**: JWT bearer token authentication with bcrypt password hashing.
- **Todo Management**: Full CRUD operations for Todo items.
- **Pagination & Search**: List filtering with pagination (`page`, `limit`), search filtering (`search`), and sorting (`sort_by`, `order`).
- **Flexible Database Backends**: Dual support for SQLite (`modernc.org/sqlite`) and PostgreSQL (`github.com/lib/pq`).
- **Observability**: Structured JSON logging powered by `zerolog` with correlation ID middleware (`X-Request-ID`).
- **Rate Limiting**: Built-in IP-based rate limiting middleware.
- **OpenAPI Specification**: Fully documented endpoints in [openapi.json](file:///home/robertheo/repository/vibecoding/todo-list-go/openapi.json).

---

## 🛠️ Technology Stack

- **Language**: Go 1.25
- **Router**: Go standard library `http.ServeMux` (Go 1.22+ pattern matching)
- **Database Drivers**:
  - PostgreSQL: `github.com/lib/pq`
  - SQLite: `modernc.org/sqlite`
- **Authentication**: `github.com/golang-jwt/jwt/v5`
- **Password Hashing**: `golang.org/x/crypto/bcrypt`
- **Logger**: `github.com/rs/zerolog`
- **Config Loader**: `github.com/joho/godotenv`

---

## 📁 Directory Structure

```
todo-list-go/
├── cmd/
│   └── main.go                 # Application entrypoint & server lifecycle
├── internal/
│   ├── config/                 # Env loader and zerolog logger configuration
│   ├── http/                   # Handlers, router setup, and middleware (auth, correlation, rate limit)
│   ├── models/                 # Domain structs & DTOs
│   ├── repository/             # Database implementations (PostgreSQL & SQLite)
│   └── service/                # Core business logic
├── pkg/
│   └── http/                   # Standardized JSON HTTP response helpers
├── openapi.json                # OpenAPI 3.0 specification
└── .env                        # Environment variables configuration
```

---

## ⚙️ Configuration (.env)

Create a `.env` file in the root directory (or use environment variables):

```env
PORT=8080
JWT_SECRET=super-secret-key-change-in-production

# Select database driver: "sqlite" or "postgres"
DB_DRIVER=postgres

# PostgreSQL Connection String (when DB_DRIVER=postgres)
DB_CONN=postgres://postgres:mysecretpassword@localhost:5432/tododb?sslmode=disable

# SQLite Path (when DB_DRIVER=sqlite)
DB_PATH=todo.db
```

---

## 🏁 Getting Started

### Prerequisites

- [Go 1.25](https://golang.org/dl/) or higher installed.
- (Optional) PostgreSQL database server running.

### Installation & Run

1. **Clone repository and install dependencies**:
   ```bash
   git clone https://github.com/robertheo15/todo-list-go.git
   cd todo-list-go
   go mod download
   ```

2. **Run Application**:
   ```bash
   go run ./cmd/main.go
   ```

---

## 🧪 Running Tests & Coverage

To run unit tests across all packages:

```bash
go test ./...
```

### Service Layer Test Coverage Enforcement

Test coverage for the business logic layer (`./internal/service/...`) is strictly required to be **$\ge 75\%$**.

* **Run Coverage Check Locally**:
  ```bash
  chmod +x ./scripts/check_service_coverage.sh
  ./scripts/check_service_coverage.sh
  ```

* **Percentage Only Command**:
  ```bash
  go test -coverprofile=coverage.out ./internal/service/... > /dev/null && go tool cover -func=coverage.out | grep total | awk '{print $3}'
  ```

* **Automated PR Enforcement**:
  A GitHub Actions workflow ([.github/workflows/coverage-check.yml](file:///.github/workflows/coverage-check.yml)) automatically runs on Pull Requests touching `internal/service/**` to ensure coverage meets or exceeds the 75% threshold before code can be merged.


---

## 📚 API Endpoints

Refer to [openapi.json](file:///home/robertheo/repository/vibecoding/todo-list-go/openapi.json) for the full OpenAPI 3.0 specification.

### Public Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/register` | Register a new user account |
| `POST` | `/login` | Authenticate user & receive JWT token |

### Protected Endpoints (Requires `Authorization: Bearer <token>`)

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/todos` | Create a new todo item |
| `GET` | `/todos` | List user todo items (supports `page`, `limit`, `search`, `sort_by`, `order`) |
| `PUT` | `/todos/{id}` | Update an existing todo item |
| `DELETE` | `/todos/{id}` | Delete a todo item |

---

## 📄 License

Distributed under the MIT License.
