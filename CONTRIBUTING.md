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

Keep the shipped HTTP surface limited to `GET /health`; do not add business API demonstrations or require consumers to implement a service/backend. Wire added dependencies explicitly in `cmd/server`. Cover the shared error envelope, sanitized internal errors, missing routes, method rejection, request IDs, and body limits with focused tests.

The template acceptance check recursively generates from the current working tree, including relevant uncommitted files. Never put credentials or personal environment files in fixtures. It verifies module/import rewriting, race tests, vet, build, skeleton HTTP behavior, and SIGTERM exit. Container checks preserve production image, non-root, filesystem, HTTP, and shutdown checks and clean up their own images and containers.

For a public tag or pushed full commit, set `RELEASE_TAG` to the exact version/commit and run `go run ./tools/publiccheck -version "$RELEASE_TAG"`. It verifies public module proxy/checksum-database availability and the generated consumer's race tests, vet, native HTTP, SIGTERM, and production container behavior. Network access and a running Linux-container Docker daemon are required; unavailable Docker fails the check. Use `templatecheck` for unpublished working-tree changes. Maintainer-authorized tag pushes run public verification before automated GitHub Release creation; manual workflow dispatch is validation only. Follow [RELEASING.md](RELEASING.md) for the release process and recovery without rewriting tags.

Report reproducible bugs with your Go version, OS/architecture, commands, expected behavior, and sanitized output. Follow [SECURITY.md](SECURITY.md) for vulnerabilities. Do not include secrets in issues, pull requests, logs, or screenshots.

Maintainers review changes before merging. Do not claim a CI, release, or platform check passed unless its actual result is available.
