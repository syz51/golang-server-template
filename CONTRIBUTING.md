# Contributing

Use Go 1.27.3 and clone this repository as described in the README. Linux and macOS are the supported test platforms. Docker checks need a running Linux-container daemon.

Keep changes focused. Include the problem, resulting behavior, and verification in your pull request. Add regression tests for behavior changes, especially configuration precedence, concurrency, error statuses, generation, and shutdown. Preserve the canonical module path `github.com/syz51/golang-server-template`; template consumers choose their own paths through `gonew`.

Before submitting:

```sh
gofmt -w cmd internal tools
go test -race ./...
go vet ./...
go mod verify
go run ./tools/templatecheck
go run ./tools/containercheck
```

Run the pinned vulnerability scanner configured in `.github/workflows/validate.yml` and explain any advisory's reachability rather than suppressing findings without evidence. If dependencies change, run `go mod tidy` and include both module files. Keep tool versions, example configuration, README defaults, and CI consistent.

The template acceptance check generates from the current working tree, including relevant uncommitted files. Never put credentials or personal environment files in fixtures. Container checks create and clean up their own images and containers.

Report reproducible bugs with your Go version, OS/architecture, commands, expected behavior, and sanitized output. Follow [SECURITY.md](SECURITY.md) for vulnerabilities. Do not include secrets in issues, pull requests, logs, or screenshots.

Maintainers review changes before merging. Do not claim a CI, release, or platform check passed unless its actual result is available.
