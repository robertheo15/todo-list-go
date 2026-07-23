# Development Plan: To-Do List API
## 0. Overview
Make sure the Restful API is not using any deprecated library.

## 1. System Architecture
Based on the provided architecture diagram (`todo-list-api.png`), the system consists of three main components:
*   **To-Do List UI**: The client application sending requests (Sign-up, Log-in, Other ops).
*   **ToDo API**: The central RESTful web service handling business logic and authentication.
*   **Database**: The persistence layer, storing two main entities: `Users` and `Tasks`.

## 2. Technology Stack
*   **Language**: Go (implied by repository name `todo-list-go`)
*   **Database**: PostgreSQL
*   **Authentication**: JWT (JSON Web Tokens) for stateless authentication.
*   **Routing**: Standard `net/http` or a router like `chi` or `gorilla/mux`.

## 3. Database Schema
We will need two main tables/collections as seen in the diagram:

### Users Table
*   `id` (Primary Key, UUID/Int)
*   `name` (String)
*   `email` (String, Unique)
*   `password_hash` (String)
*   `created_at` (Timestamp)

### Tasks (To-Do Items) Table
*   `id` (Primary Key, UUID/Int)
*   `user_id` (Foreign Key -> Users.id)
*   `title` (String)
*   `description` (Text)
*   `created_at` (Timestamp)
*   `updated_at` (Timestamp)

## 4. API Endpoints Implementation

### Authentication
*   `POST /register`: Accepts `name`, `email`, and `password`. Hashes password, saves to DB, returns JWT.
*   `POST /login`: Accepts `email` and `password`. Verifies credentials, returns JWT.

### To-Do (Tasks) CRUD
*(Requires JWT Authentication Header: `Authorization: Bearer <token>`)*
*   `POST /todos`: Create a new task (`title`, `description`).
*   `GET /todos`: Retrieve tasks for the authenticated user. Includes pagination (`page`, `limit`).
*   `PUT /todos/:id`: Update an existing task (`title`, `description`). Ensures the user owns the task (403 Forbidden otherwise).
*   `DELETE /todos/:id`: Delete a task. Ensures user owns the task.

## 5. Development Phases

### Phase 1: Setup and Configuration
*   Initialize Go module (`go mod init`).
*   Set up database connection and migrations.
*   Set up environment variables for DB credentials and JWT secret.

### Phase 2: User Authentication
*   Implement `Users` repository/model.
*   Create `/register` and `/login` handlers.
*   Implement password hashing (e.g., using `bcrypt`).
*   Implement JWT token generation.
*   Create an Authentication Middleware to protect routes.

### Phase 3: To-Do Items Management
*   Implement `Tasks` repository/model.
*   Create CRUD handlers for `/todos` route.
*   Implement ownership validation in Update/Delete endpoints.
*   Implement pagination for the `GET /todos` endpoint.

### Phase 4: Refinement and Bonus Features
*   **Data Validation**: Add rigorous validation for incoming JSON payloads.
*   **Filtering and Sorting**: Extend `GET /todos` to accept sorting parameters and filters (e.g., status).
*   **Rate Limiting**: Implement a rate limiter middleware to prevent abuse.
*   **Authorization**: Others user can't edit someone todo and view.
*   **Testing**: Write unit tests for API endpoints and services.
*   **Refresh Tokens**: Implement a refresh token mechanism for better security.
