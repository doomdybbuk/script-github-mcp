# Stage 1: build
FROM golang:1.24 AS builder
ARG VERSION="dev"
WORKDIR /src

# Copy repo (assumes this Dockerfile is at repo root)
COPY . .

# Build official MCP server binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=${VERSION}" -o /bin/github-mcpserver ./cmd/github-mcp-server

# Build http proxy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /bin/http-proxy ./cmd/http-proxy

# Stage 2: runtime image
FROM gcr.io/distroless/base-debian12
WORKDIR /app

# Copy binaries
COPY --from=builder /bin/github-mcpserver /app/github-mcpserver
COPY --from=builder /bin/http-proxy /app/http-proxy

# Expose port used by http-proxy
EXPOSE 8080

# Entry: run http-proxy and let it spawn the stdio server
ENTRYPOINT ["/app/http-proxy", "--addr=0.0.0.0:8080", "--server-cmd", "/app/github-mcpserver stdio", "--timeout=30"]
