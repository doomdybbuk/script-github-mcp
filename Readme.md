# github-mcp-http-proxy

**HTTP → stdio bridge for GitHub MCP server + Streamlit helpers**

This repo bundles:
- the official `github-mcp-server` (the MCP "stdio" server that executes toollike JSON actions)
- an `http-proxy` binary that spawns the MCP server and exposes a simple HTTP endpoint (`/call`) so you can POST MCP commands (`create_repository`, `multi_file`, `create_or_update_file`, etc.)
- a Streamlit UI (optional) to help download a project (eg from Google Drive), package it, generate a CodeQL workflow via Azure OpenAI, and send payloads to the MCP HTTP proxy.

---

## Overview: what this does and why

GitHub's MCP server (`github-mcp-server`) accepts JSON tool calls over **stdio**. That's great for command-line automation or local agents, but inconvenient for GUIs and web apps. This project provides a small HTTP bridge (`http-proxy`) so you can call MCP operations using a simple HTTP endpoint (`POST http://localhost:8080/call`). The bridge spawns the stdio server and forwards requests/responses.

Common uses:
- Build automation agents that create repos and push multi-file commits
- Auto-generate and add CodeQL workflow YAML to repos (using Azure OpenAI)
- Import a zipped project from Google Drive and push it to GitHub automatically

---

## Prerequisites

- Go (1.20+ recommended). For the Dockerfile we use Go 1.24.
- Docker (optional; for containerized runs)
- Git
- `pip` + `certifi` (or use `pip install pip-system-certs` on some Windows installs if you see certificate issues)
- A GitHub Personal Access Token (PAT) with appropriate scopes (repo and workflow/write if necessary)
- If using Azure OpenAI for generating CodeQL workflows: Azure OpenAI endpoint, key and deployment name.

Environment variables used:
- `GITHUB_PERSONAL_ACCESS_TOKEN` — required to perform GitHub API actions
- `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_KEY`, `AZURE_DEPLOYMENT` — optional; used to generate CodeQL via Azure OpenAI

---

## Repo layout

github-mcp-http-proxy/
├── cmd/
│ ├── github-mcp-server/ # upstream official server (or submodule)
│ └── http-proxy/ # this project's http → stdio bridge
├── streamlit_app.py # Streamlit UI (optional)
├── github_mcp_protocol.md # protocol reference + CodeQL template markers
├── Dockerfile # builds both binaries and runs http-proxy
└── README.md # this file

yaml
Copy code

---

## Quickstart — Build & run locally

### Windows (PowerShell) or Linux/macOS (Bash) — same steps, minor differences noted

> **1. Clone the repo**
```bash
git clone https://github.com/<youruser>/github-mcp-http-proxy.git
cd github-mcp-http-proxy
2. Initialize module (one-time if not already)

powershell
Copy code
# Windows PowerShell
go mod init github.com/<youruser>/github-mcp-http-proxy
bash
Copy code
# Linux/macOS
go mod init github.com/<youruser>/github-mcp-http-proxy
Why? go.mod tracks module path & dependencies for go build.

3. Build the MCP server binary

powershell
Copy code
# Windows
go build -o github-mcpserver.exe ./cmd/github-mcp-server
bash
Copy code
# Linux/macOS
go build -o github-mcpserver ./cmd/github-mcp-server
What it does: compiles the official MCP server (the component that executes tool JSON via stdio).

4. Build the HTTP proxy binary

powershell
Copy code
# Windows
go build -o http-proxy.exe ./cmd/http-proxy
bash
Copy code
# Linux/macOS
go build -o http-proxy ./cmd/http-proxy
What it does: compiles the HTTP-to-stdio bridge that listens on a TCP port and spawns the MCP server.

5. Run the HTTP bridge (which spawns the MCP server)

powershell
Copy code
# Windows
.\http-proxy.exe --addr="0.0.0.0:8080" --server-cmd ".\github-mcpserver.exe stdio" --timeout=30
bash
Copy code
# Linux/macOS
./http-proxy --addr="0.0.0.0:8080" --server-cmd "./github-mcpserver stdio" --timeout=30
Flags explained

--addr="0.0.0.0:8080" — the HTTP address to listen on. 0.0.0.0 binds all interfaces; use 127.0.0.1:8080 to bind only localhost.

--server-cmd "./github-mcpserver stdio" — the command used by the proxy to launch the MCP server. stdio tells MCP server to read tool requests from stdin and respond on stdout (the proxy pipes data).

--timeout=30 — per-request timeout in seconds for MCP responses; prevents hanging requests.

After running, your MCP endpoint is available at:

bash
Copy code
http://localhost:8080/call

You can POST MCP JSON to this endpoint.

Docker — build & run

Dockerfile (already in repo) builds both the github-mcpserver and http-proxy binaries, then runs the http-proxy in the container.

Build image

docker build -t github-mcp-http-proxy:latest .


Run container

docker run -p 8080:8080 \
  -e GITHUB_PERSONAL_ACCESS_TOKEN="${GITHUB_PERSONAL_ACCESS_TOKEN}" \
  github-mcp-http-proxy:latest


Notes

You can pass GITHUB_PERSONAL_ACCESS_TOKEN as an env var to the container.

If you need to run Azure OpenAI usage inside the container (for CodeQL generation), pass AZURE_OPENAI_ENDPOINT, AZURE_OPENAI_KEY, AZURE_DEPLOYMENT likewise.

The container will expose the HTTP proxy on port 8080. You can then POST to http://<host-ip>:8080/call.

Example MCP JSON payloads (save to examples/ and curl them)

Replace your-github-username and my-repo where needed.

Create repository

examples/create_repo.json

{
  "tool": "create_repository",
  "arguments": {
    "name": "my-hello-java",
    "description": "Imported from Drive via MCP UI",
    "private": false,
    "autoInit": true
  }
}


Curl:

curl -X POST "http://localhost:8080/call" \
  -H "Content-Type: application/json" \
  -d @examples/create_repo.json

Create a branch

examples/create_branch.json

{
  "tool": "create_branch",
  "arguments": {
    "owner": "your-github-username",
    "repo": "my-hello-java",
    "branch": "feature-1",
    "base_branch": "main"
  }
}


Curl similar to above.

Create/update a single file (README)

examples/create_file.json

{
  "tool": "create_or_update_file",
  "arguments": {
    "owner": "your-github-username",
    "repo": "my-hello-java",
    "path": "README.md",
    "message": "Add README",
    "content": "SGVsbG8gV29ybGQh" ,
    "branch": "main"
  }
}


SGVsbG8gV29ybGQh is Hello World! base64 encoded.

Multi-file commit (atomic) — multi_file

examples/multi_file.json

{
  "tool": "multi_file",
  "arguments": {
    "owner": "your-github-username",
    "repo": "my-hello-java",
    "branch": "main",
    "commit_message": "Add project files",
    "files": [
      { "path": "pom.xml", "content": "..." },
      { "path": "src/main/java/com/example/App.java", "content": "..." }
    ]
  }
}


content should be base64 encoded file contents (no line breaks).

Important: Some MCP server builds do not include a multi_file tool by default. If tool 'multi_file' not found occurs, either:

Include/enable the multi_file tool in the MCP server build; or

Fall back to multiple create_or_update_file calls for each file (slower but works).
