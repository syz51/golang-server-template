# Optional development debugger; build from the repository root.
# docker build -f tools/debugcheck/Dockerfile -t server-debug .
# docker run --rm --cap-drop=ALL -p 127.0.0.1:8080:8080 -p 127.0.0.1:2345:2345 server-debug
# Starts paused; attach a client and continue to start HTTP serving.
FROM golang:1.27.3-alpine
WORKDIR /src
RUN go install github.com/go-delve/delve/cmd/dlv@v1.27.2
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY configs ./configs
RUN go build -gcflags="all=-N -l" -o /debug/server ./cmd/server
ENV APP_SERVER_HOST=0.0.0.0 APP_SERVER_PORT=8080 \
    APP_SERVER_READ_TIMEOUT=1h APP_SERVER_WRITE_TIMEOUT=1h HOME=/tmp
USER 65532:65532
EXPOSE 8080 2345
ENTRYPOINT ["dlv", "exec", "/debug/server", "--headless", "--listen=0.0.0.0:2345", "--api-version=2", "--accept-multiclient"]
