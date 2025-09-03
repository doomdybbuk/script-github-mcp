# github-mcp-http-proxy

**HTTP → stdio bridge for GitHub MCP server**

---

## What is GitHub MCP Server?

GitHub’s **Model Context Protocol (MCP) server** is a lightweight server that exposes JSON-based tool actions (like `create_repository`, `create_branch`, `create_or_update_file`, etc.).

By default, the official GitHub MCP server only runs over **stdio**—meaning it reads/writes JSON over standard input/output pipes.  
This works for **local CLI tools or agents**, but is inconvenient for GUIs, web apps, or network clients.

---

## What this project adds

This repo bundles:

- The official `github-mcp-server` (the stdio-only server)
- **An HTTP-proxy bridge** that spawns the MCP server and exposes it over a simple HTTP endpoint (`/call`)

So instead of dealing with stdio, you can now just do:
curl -X POST http://localhost:8080/call -d @payload.json

text

---

## How it works

- The `http-proxy` binary runs a local HTTP server (default: `0.0.0.0:8080`).
- On each request, it spawns the `github-mcp-server` with stdio mode, pipes the request in, and returns the response over HTTP.

---

## Build & Run Locally

**1. Clone repo**
git clone https://github.com/<youruser>/github-mcp-http-proxy.git
cd github-mcp-http-proxy

text

**2. Init Go module**
go mod init github.com/<youruser>/github-mcp-http-proxy

text
Tracks dependencies for builds.

**3. Build MCP server**
- *Windows*
    ```
    go build -o github-mcpserver.exe ./cmd/github-mcp-server
    ```
- *Linux/macOS*
    ```
    go build -o github-mcpserver ./cmd/github-mcp-server
    ```
This compiles the official stdio MCP server.

**4. Build HTTP proxy**
- *Windows*
    ```
    go build -o http-proxy.exe ./cmd/http-proxy
    ```
- *Linux/macOS*
    ```
    go build -o http-proxy ./cmd/http-proxy
    ```
This builds the HTTP → stdio bridge.

**5. Run proxy (spawns MCP server)**
- *Windows*
    ```
    .\http-proxy.exe --addr="0.0.0.0:8080" --server-cmd ".\github-mcpserver.exe stdio" --timeout=30
    ```
- *Linux/macOS*
    ```
    ./http-proxy --addr="0.0.0.0:8080" --server-cmd "./github-mcpserver stdio" --timeout=30
    ```

**Flags explained:**
- `--addr=0.0.0.0:8080` → HTTP server bind address
- `--server-cmd "./github-mcpserver stdio"` → how proxy launches the MCP stdio server
- `--timeout=30` → per-request timeout in seconds

---

## Docker Setup

A `Dockerfile` is included that builds both binaries and runs the HTTP proxy.

**Build image**
docker build -t github-mcp-http-proxy:latest .

text

**Run container**
docker run -p 8080:8080
-e GITHUB_PERSONAL_ACCESS_TOKEN="${GITHUB_PERSONAL_ACCESS_TOKEN}"
github-mcp-http-proxy:latest

text
The server will now be reachable at:  
[http://localhost:8080/call](http://localhost:8080/call)

---

## Example JSON Payloads

#### Create Repository

examples/create_repo.json:
{
"tool": "create_repository",
"arguments": {
"name": "my-hello-java",
"description": "Imported via MCP HTTP bridge",
"private": false,
"autoInit": true
}
}

text
Run:
curl -X POST http://localhost:8080/call
-H "Content-Type: application/json"
-d @examples/create_repo.json

text

#### Create Branch

{
"tool": "create_branch",
"arguments": {
"owner": "your-username",
"repo": "my-hello-java",
"branch": "feature-1",
"base_branch": "main"
}
}

text

#### Create/Update File

{
"tool": "create_or_update_file",
"arguments": {
"owner": "your-username",
"repo": "my-hello-java",
"branch": "main",
"path": "README.md",
"message": "Add README",
"content": "SGVsbG8gV29ybGQh"
}
}

text
*(content is base64 for "Hello World!")*

#### Multi-File Commit

{
"tool": "multi_file",
"arguments": {
"owner": "your-username",
"repo": "my-hello-java",
"branch": "main",
"commit_message": "Add project files",
"files": [
{ "path": "pom.xml", "content": "..." },
{ "path": "src/main/java/com/example/App.java", "content": "..." }
]
}
}

text
:warning: *Note: some MCP builds may not support `multi_file` out of the box. If you see `tool 'multi_file' not found`, either patch MCP to add it or fall back to multiple `create_or_update_file` calls.*

---

## Why this matters

This simple bridge turns the **stdio-only GitHub MCP server into an HTTP API** you can call with **curl, Postman, or any client library**.

It unlocks easy integration with **automation scripts, CI/CD, dashboards, and any HTTP-capable service**.

---
