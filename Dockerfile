FROM golang:1.27.3-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /server ./cmd/server

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
ENV APP_APP_ENVIRONMENT=production
COPY --from=builder /server /app/server
COPY --from=builder /app/configs /app/configs
COPY --from=builder /app/LICENSE /licenses/LICENSE
EXPOSE 8080
CMD ["/app/server"]
