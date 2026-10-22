# Go Server Template

An Echo v5 HTTP server starter with validated configuration, structured logging, request limits, graceful shutdown, tests, and a non-root Docker image. Generate a new Go module with `gonew`, then replace the demonstration API with your application.

The included users API is an **unauthenticated, in-memory demonstration**. Data disappears on restart and is not shared between instances. `/health` reports process liveness only. Add application-specific authentication, authorization, durable storage, and dependency readiness checks before using the example to serve real user data.

## Requirements and support

- Go **1.27.3** or newer. CI validates Go 1.27.3 on Linux and macOS.
- Docker with a running Linux-container daemon for container checks. Buildx is optional.
- Air and Delve are optional development tools.

Windows is not currently part of the supported test matrix. Containers target Linux and build for the Docker builder's native architecture; cross-platform images require an explicitly configured cross-build workflow.

## Generate an application

Run the pinned [gonew tool](https://pkg.go.dev/golang.org/x/tools/cmd/gonew) from the directory that should contain your new project:

```sh
go run golang.org/x/tools/cmd/gonew@v0.52.0 \
  github.com/syz51/golang-server-template@latest \
  example.com/my-server
cd my-server
cp .env.example .env
go test ./...
go run ./cmd/server
```

Choose your own module path, such as `github.com/your-name/my-server`. `gonew` rewrites `go.mod` and internal Go imports automatically; **do not run a second module-renaming step**. For reproducible generation, replace `@latest` with an existing tag from [Releases](https://github.com/syz51/golang-server-template/releases). The template maintainer's release process is in [RELEASING.md](RELEASING.md).

In another terminal:

```sh
curl --fail http://localhost:8080/health
```

Set `APP_APP_NAME` and `APP_APP_VERSION` for your application. Replace this README's upstream repository links and the contribution/security/release policies with your own policies when adopting the template. Retain the upstream MIT license notice. Generated projects do not automatically receive future template fixes; review upstream release changes and apply relevant updates.

## Work on the template itself

Clone the repository to contribute or validate unpublished changes:

```sh
git clone https://github.com/syz51/golang-server-template.git
cd golang-server-template
go mod download
go test -race ./...
go run ./cmd/server
```

A direct clone intentionally retains the upstream module identity. Use the generation workflow above for a new application. Maintainers can validate generation from the current working tree without publishing it:

```sh
go run ./tools/templatecheck
```

## Configuration

Precedence, from highest to lowest:

1. Exported process environment variables.
2. `.env` in the working directory.
3. `config.yaml` in the working directory, otherwise `configs/config.yaml`.
4. Built-in defaults.

Each load is isolated. Loading `.env` does not change the process environment. Missing files are optional; malformed files and invalid supported settings fail startup. No configuration is read from your home directory. Configuration paths are relative to the process working directory, not the binary location.

The complete file is [configs/config.yaml](configs/config.yaml), and all environment keys are in [.env.example](.env.example). Unused database and debug settings have been removed: the starter does not connect to a database.

| Setting | Default | Environment variable |
| --- | --- | --- |
| Listen host | `0.0.0.0` | `APP_SERVER_HOST` |
| Listen port | `8080` | `APP_SERVER_PORT` |
| Maximum request body | `1048576` bytes | `APP_SERVER_MAX_BODY_BYTES` |
| Header timeout | `5s` | `APP_SERVER_READ_HEADER_TIMEOUT` |
| Request read timeout | `30s` | `APP_SERVER_READ_TIMEOUT` |
| Response write timeout | `30s` | `APP_SERVER_WRITE_TIMEOUT` |
| Idle timeout | `60s` | `APP_SERVER_IDLE_TIMEOUT` |
| Graceful shutdown timeout | `30s` | `APP_SERVER_SHUTDOWN_TIMEOUT` |
| Allowed CORS origins | empty; disabled | `APP_SERVER_CORS_ALLOWED_ORIGINS` |
| Log level | `info` | `APP_LOGGER_LEVEL` |
| Log format | `json` | `APP_LOGGER_FORMAT` |
| Application name | `golang-server-template` | `APP_APP_NAME` |
| Application version | `1.0.0` | `APP_APP_VERSION` |
| Environment label | `development` | `APP_APP_ENVIRONMENT` |

Durations use Go strings such as `5s` or `1m`; byte limits and timeouts must be positive. Log levels are `debug`, `info`, `warn`, and `error`; formats are `json` and `text`. Environment labels are `development`, `staging`, and `production`. The label identifies deployment configuration; it does not add authentication or persistent storage.

CORS is opt-in. Supply a YAML list or comma-separated environment value, for example `APP_SERVER_CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com`. IPv6 literals such as `APP_SERVER_HOST=::1` are supported without brackets.

Request bodies are buffered up to the configured limit before decoding. Oversized bodies, including chunked bodies, return HTTP 413. Streaming uploads or SSE require deliberate changes to buffering and timeout policies.

## Demonstration API

```sh
curl --fail -X POST http://localhost:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"john@example.com","first_name":"John","last_name":"Doe","age":25,"phone":"+1234567890"}'

curl --fail http://localhost:8080/api/v1/users/1

curl --fail -X PUT http://localhost:8080/api/v1/users/1 \
  -H 'Content-Type: application/json' -d '{"first_name":"Jane","phone":""}'

curl --fail 'http://localhost:8080/api/v1/users?page=1&per_page=10'

curl --fail -X DELETE http://localhost:8080/api/v1/users/1
```

| Route | Successful response |
| --- | --- |
| `GET /health` | 200; `status`, `service`, `version`, `timestamp` |
| `POST /api/v1/users` | 201; created user |
| `GET /api/v1/users/:id` | 200; user |
| `PUT /api/v1/users/:id` | 200; updated user |
| `DELETE /api/v1/users/:id` | 204; empty body |
| `GET /api/v1/users` | 200; `users` and pagination `meta` |

Updates are partial despite the example's `PUT` route. Omitted or `null` phone leaves the value unchanged; an empty string clears it. Nonempty phone values must use E164 format. Missing users return 404; duplicate emails return 409; invalid inputs return 400; unsupported request media types return 415.

Pagination is ordered by ascending user ID. Defaults are page 1 and 10 users per page. `per_page` must be 1–100; invalid, empty, repeated, or overflowing pagination parameters return 400. Valid pages beyond the available users return 200 with `users: []`. Pagination is not a snapshot across concurrent writes.

## Development and checks

```sh
go build -o ./bin/server ./cmd/server
go run ./cmd/server
go test -race ./...
go test -coverpkg=./internal/...,./cmd/server -cover ./...
go vet ./...
go mod verify
go run ./tools/templatecheck
go run ./tools/containercheck
```

CI runs formatting, vet, race tests, module verification, a vulnerability scan, generated-consumer checks, and container checks. Local Docker checks require a running daemon; generation checks need network access for pinned Go tools and dependencies. A clean vulnerability scan is a point-in-time result, not a security guarantee.

Install the pinned optional hot-reload tool:

```sh
go install github.com/air-verse/air@v1.67.4
air
```

Put `$(go env GOPATH)/bin` or your `GOBIN` on `PATH`. Air runs the application directly and reloads Go, YAML, and `.env` changes. It does not require a debugger.

For an interactive debugging session, stop Air first and run:

```sh
go install github.com/go-delve/delve/cmd/dlv@v1.27.2
dlv debug ./cmd/server
```

For a remote IDE attachment, use `dlv debug ./cmd/server --headless --listen=127.0.0.1:2345 --api-version=2 --accept-multiclient --continue`, then attach using host `127.0.0.1` and port `2345`. Keep the debug listener private. Delve requires the operating system's debugger permissions and setup; consult its [installation guide](https://github.com/go-delve/delve/blob/v1.27.2/Documentation/installation/README.md). Debugger startup is separate from the standard Air workflow.

## Docker

```sh
docker build -t my-server .
docker run --rm --name my-server -p 127.0.0.1:8080:8080 \
  -e APP_APP_ENVIRONMENT=production -e APP_APP_NAME=my-server my-server
```

The image contains a static Linux binary, the default YAML, and `/licenses/LICENSE`. It runs as a non-root user with a distroless base and has no shell. `.dockerignore` excludes local environment files and Git history from the build context. Do not bake secrets into YAML or build arguments.

Use `--env-file .env` to supply a local environment file, or mount configuration read-only at `/app/configs/config.yaml`. For production, provide deployment-specific settings through your platform's configuration/secret mechanism. Docker defaults to the production environment label; an explicit environment file can override it.

Stop with enough grace for active requests:

```sh
docker stop --time 35 my-server
```

The process handles SIGINT and SIGTERM. Coordinate the platform stop grace period with `APP_SERVER_SHUTDOWN_TIMEOUT`, allowing additional scheduling time. Graceful shutdown drains active HTTP requests and exits nonzero if draining exceeds its timeout. Inspect logs with `docker logs my-server`; use an appropriate debug container when needed, because `docker exec ... sh` cannot work in the distroless image.

## Project layout and extension

- `cmd/server`: composition, middleware, routes, logging, and server lifecycle.
- `internal/config`: isolated, validated runtime configuration.
- `internal/handler`: HTTP decoding, validation, and error/status mapping.
- `internal/model`: request/response types and validation.
- `internal/service`: concurrency-safe demonstration business logic and storage.
- `tools`: template-generation and Docker acceptance checks.

To add an entity, define request/response types, implement its service, add handlers, and register routes in `setupRoutes`. Echo v5 handlers accept `*echo.Context`. Keep stored objects private and return detached values; use typed or sentinel service errors rather than matching error text. Add tests for failure paths and concurrent access. Introduce a persistence interface when your application needs a real backend, and make readiness checks reflect that backend's actual state.

## Contributing, security, and license

See [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), and [RELEASING.md](RELEASING.md). This project uses the [MIT License](LICENSE).
