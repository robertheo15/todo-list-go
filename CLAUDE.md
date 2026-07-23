# Weather API Developer Guidelines (CLAUDE.md)

This file contains quick development commands and guidelines for AI coding assistants (like Claude, Antigravity, etc.) working on this repository.

> [!IMPORTANT]
> **Complete Guidelines & Architecture**:
> For the comprehensive development rules, clean architecture guidelines, structured logging requirements, and refactoring guardrails, please refer to the main instruction file:
> [AGENT.md](file:///home/robertheo/repository/vibecoding/weather-api/AGENT.md)

## Development Commands

### Running Locally
- Run the server: `go run cmd/main.go`
- Build executable: `go build -o weather-api cmd/main.go`

### Testing
- Run all tests: `go test -v ./...`
- Run all tests with race detector: `go test -v -race ./...`
- Run service layer tests: `go test -v -race ./internal/service/...`

### Formatting & Linting
- Format code: `make fmt` (runs `go fmt`)
- Lint code: `make lint` (runs `golangci-lint`)
- General check: `go vet ./...`

### Docker / Docker Compose
- Start services (API + Redis): `docker-compose up --build -d`
- Stop services: `docker-compose down`
- View API logs: `docker-compose logs -f weather-api`

## Repository Structure & Custom Agents
- **Skills Directory**: Reusable playbooks and coding skills are located in [.agents/skills/](file:///home/robertheo/repository/vibecoding/weather-api/.agents/skills/).
- **Project Rules**: Custom behavior rules for the agent should be placed in `.agents/rules/` if applicable.
