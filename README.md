# Go Server Template

An opinionated production-ready Go server template using Echo, Viper, and Validator v10, following Go best practices for project layout.

## 📋 Table of Contents

- [Features](#-features)
- [Project Structure](#-project-structure)
- [Requirements](#-requirements)
- [Quick Start](#-quick-start)
- [Configuration](#%EF%B8%8F-configuration)
- [API Endpoints](#-api-endpoints)
- [Development](#-development)
- [Testing](#-testing)
- [Deployment](#-deployment)
- [Contributing](#-contributing)
- [Acknowledgments](#-acknowledgments)

## ✨ Features

- **Echo Web Framework**: High performance, minimalist Go web framework
- **Viper Configuration**: Flexible configuration management with support for multiple formats
- **Validator v10**: Comprehensive struct and field validation
- **Clean Architecture**: Following Go project layout best practices
- **Docker Support**: Multi-stage builds with Google's distroless images for enhanced security and minimal footprint
- **Hot Reload**: Air integration for development
- **Debugging Support**: Integrated Delve debugger with Air for seamless debugging experience
- **Testing**: Example unit tests with testify
- **Graceful Shutdown**: Proper server shutdown handling
- **Middleware**: Built-in middleware for logging, recovery, CORS, etc.
- **Health Check**: Health endpoint for monitoring

## 📁 Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── handler/
│   │   ├── handler.go       # HTTP handlers
│   │   └── handler_test.go  # Handler tests
│   ├── middleware/
│   │   └── middleware.go    # Custom middleware
│   ├── model/
│   │   └── user.go          # Data models and validation
│   └── service/
│       └── user.go          # Business logic
├── configs/
│   └── config.yaml          # Configuration file
├── .env.example             # Environment variables example
├── .gitignore               # Git ignore file
├── .air.toml                # Configuration file for air
├── Dockerfile               # Docker configuration
├── go.mod                   # Go modules
├── go.sum                   # Go modules checksum
└── README.md                # This file
```

## 🔧 Requirements

- Go 1.24 or higher
- Docker (optional, for containerization)
- Air (optional, for hot reload)

## 🚀 Quick Start

1. **Clone the template**:

   ```bash
   # Using gonew (recommended)
   gonew github.com/your-org/your-project my-new-project
   cd my-new-project
   
   # Or clone directly
   git clone https://github.com/your-org/your-project.git
   cd your-project
   ```

2. **Update module name**:

   ```bash
   # Replace "github.com/your-org/your-project" with your actual module path
   go mod edit -module github.com/your-username/your-project
   find . -type f -name "*.go" -exec sed -i 's|github.com/your-org/your-project|github.com/your-username/your-project|g' {} +
   ```

3. **Install dependencies**:

   ```bash
   go mod download
   ```

4. **Run the application**:

   ```bash
   go run cmd/server/main.go
   ```

5. **Test the API**:

   ```bash
   curl http://localhost:8080/health
   ```

## ⚙️ Configuration

The application supports configuration through multiple sources (in order of precedence):

1. Environment variables (prefixed with `APP_`)
2. Configuration file (`config.yaml`)
3. Default values

### Environment Variables

Copy `.env.example` to `.env` and adjust the values:

```bash
cp .env.example .env
```

### Configuration File

Edit `configs/config.yaml`:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  driver: "postgres"
  host: "localhost"
  port: 5432
  username: "postgres"
  password: "postgres"
  database: "app_db"
  ssl_mode: "disable"

logger:
  level: "info"
  format: "json"

app:
  name: "golang-server-template"
  version: "1.0.0"
  environment: "development"
  debug: true
```

## 🛠 API Endpoints

### Health Check

```http
GET /health
```

Response:

```json
{
  "status": "ok",
  "service": "golang-server-template",
  "version": "1.0.0",
  "timestamp": "2024-01-01T00:00:00Z",
  "checks": {
    "database": "ok",
    "memory": "ok"
  }
}
```

### Users API

#### Create User

```http
POST /api/v1/users
Content-Type: application/json

{
  "email": "user@example.com",
  "first_name": "John",
  "last_name": "Doe",
  "age": 25,
  "phone": "+1234567890"
}
```

#### Get User

```http
GET /api/v1/users/{id}
```

#### Update User

```http
PUT /api/v1/users/{id}
Content-Type: application/json

{
  "first_name": "Jane",
  "age": 26
}
```

#### Delete User

```http
DELETE /api/v1/users/{id}
```

#### List Users

```http
GET /api/v1/users?page=1&per_page=10
```

## 🔧 Development

### Available Commands

```bash
go build cmd/server/main.go           # Build the application
go run cmd/server/main.go             # Run the application
go test ./...                         # Run tests
go fmt ./...                          # Format code
go vet ./...                          # Run go vet
golangci-lint run                     # Run golangci-lint
go clean                              # Clean build artifacts
go mod download                       # Download dependencies
air                                   # Run with hot reload (requires air)
```

### Hot Reload Development

Install Air for hot reloading:

```bash
# Install Air version 1.61.7, until https://github.com/air-verse/air/issues/775 is fixed
go install github.com/cosmtrek/air@1.61.7

# Run with hot reload
air
```

### Debugging (Optional)

This project is configured to support debugging through Delve when using Air. The setup allows you to attach a debugger while enjoying hot reload functionality.

#### Prerequisites

Install Delve if you haven't already:

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

#### Usage

When you run `air`, the application starts with Delve debugging enabled and listens on port `2345`. You can attach your debugger to this port.

**VS Code Setup:**

Add this configuration to your `.vscode/launch.json`:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Attach to Air",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 2345,
            "host": "127.0.0.1"
        }
    ]
}
```

**GoLand/IntelliJ Setup:**

1. Go to **Run** → **Edit Configurations**
2. Add **Go Remote** configuration
3. Set **Host**: `127.0.0.1`
4. Set **Port**: `2345`

**Steps:**

1. Start the development server:

   ```bash
   air
   ```

2. Set your breakpoints in your Go code

3. Attach your debugger using the configuration above

4. The debugger will connect and you can debug while Air handles hot reloading

#### Notes

- The debug server accepts multiple client connections (`--accept-multiclient`)
- Air will rebuild and restart the debug session on file changes
- You may need to reconnect your debugger after hot reloads

## 🧪 Testing

Run all tests:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -v -cover ./...
```

Run tests for specific package:

```bash
go test -v ./internal/handler
```

## 🐳 Deployment

### Docker

This project uses a **multi-stage Docker build** with **Google's distroless images** for optimal security, performance, and image size.

#### Docker Build Features

- **Multi-stage build**: Separates build environment from runtime environment
- **Distroless base image**: `gcr.io/distroless/static-debian12:nonroot` for minimal attack surface
- **Build caching**: Optimized layer caching for faster builds
- **Non-root user**: Runs as non-privileged user for enhanced security
- **Static binary**: CGO-disabled build for maximum compatibility

#### Quick Start

Build and run with Docker:

```bash
# Build the image
docker build -t golang-server-template .

# Run with environment file
docker run -p 8080:8080 --env-file .env golang-server-template

# Run with environment variables
docker run -p 8080:8080 \
  -e APP_SERVER_PORT=8080 \
  -e APP_DATABASE_HOST=localhost \
  golang-server-template
```

#### Advanced Docker Commands

```bash
# Build with custom tag
docker build -t your-registry/golang-server-template:v1.0.0 .

# Run with volume mount for configs
docker run -p 8080:8080 \
  -v $(pwd)/configs:/app/configs:ro \
  golang-server-template

# Run with custom network
docker network create app-network
docker run -p 8080:8080 \
  --network app-network \
  --name golang-server \
  golang-server-template

# View container logs
docker logs golang-server

# Execute shell in running container (for debugging)
docker exec -it golang-server sh
```

#### Docker Compose (Optional)

Create a `docker-compose.yml` for local development:

```yaml
version: '3.8'
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - APP_SERVER_PORT=8080
      - APP_DATABASE_HOST=postgres
    depends_on:
      - postgres
    volumes:
      - ./configs:/app/configs:ro
  
  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=app_db
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

Run with Docker Compose:

```bash
docker-compose up -d
```

#### Distroless Benefits

**Security Advantages:**

- No shell, package managers, or unnecessary binaries
- Minimal attack surface with only essential runtime dependencies
- Non-root user execution
- No known CVEs from base OS packages

**Performance Benefits:**

- Smaller image size (~15-20MB vs ~100MB+ with full OS)
- Faster container startup and deployment
- Reduced bandwidth for image pulls
- Lower memory footprint

**Production Considerations:**

- Debugging requires tools like `docker exec` with debug containers
- No shell access (use `kubectl debug` or `docker run --rm -it --pid container:xyz --net container:xyz --cap-add SYS_PTRACE nicolaka/netshoot` for debugging)

#### Image Size Comparison

```bash
# Check final image size
docker images golang-server-template

# Expected output:
# REPOSITORY              TAG       SIZE
# golang-server-template  latest    ~15-20MB
```

### Production Build

Build optimized binary for production:

```bash
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -o server cmd/server/main.go
```

## 📝 Adding New Features

### Adding a New Entity

1. **Create model** in `internal/model/`:

   ```go
   type YourEntity struct {
       ID   int    `json:"id" validate:"-"`
       Name string `json:"name" validate:"required,min=2,max=50"`
   }
   ```

2. **Create service** in `internal/service/`:

   ```go
   type YourEntityService struct {
       // business logic
   }
   ```

3. **Create handlers** in `internal/handler/`:

   ```go
   func (h *Handler) CreateYourEntity(c echo.Context) error {
       // handler logic
   }
   ```

4. **Add routes** in `cmd/server/main.go`:

   ```go
   entities := api.Group("/entities")
   entities.POST("", h.CreateYourEntity)
   ```

### Adding Middleware

Create custom middleware in `internal/middleware/`:

```go
func YourMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // middleware logic
            return next(c)
        }
    }
}
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Echo](https://echo.labstack.com/) - High performance, minimalist Go web framework
- [Viper](https://github.com/spf13/viper) - Go configuration with fangs
- [Validator](https://github.com/go-playground/validator) - Go Struct and Field validation
- [Air](https://github.com/cosmtrek/air) - Live reload for Go apps
- [Delve](https://github.com/go-delve/delve) - Debugger for the Go programming language
