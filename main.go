package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Request body expected from HTTP client
type callRequestBody struct {
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	// optional override for the command to run the stdio server; e.g. "./github-mcp-server stdio"
	ServerCmd string `json:"server_cmd,omitempty"`
	// optional GitHub token override for this request
	GithubPAT string `json:"github_pat,omitempty"`
}

// JSON-RPC request structure used by MCP server
type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  requestParams `json:"params"`
}

type requestParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

func randomID() int {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return int(n.Int64())
}

func buildJSONRPCPayload(tool string, args map[string]interface{}) ([]byte, error) {
	rpc := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      randomID(),
		Method:  "tools/call",
		Params: requestParams{
			Name:      tool,
			Arguments: args,
		},
	}
	return json.Marshal(rpc)
}

func callHandler(defaultServerCmd string, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// decode request body
		var body callRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.Tool == "" {
			http.Error(w, "`tool` is required (e.g. create_issue)", http.StatusBadRequest)
			return
		}

		// build JSON-RPC payload
		payload, err := buildJSONRPCPayload(body.Tool, body.Arguments)
		if err != nil {
			http.Error(w, "failed to marshal jsonrpc payload: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// choose server command
		serverCmdStr := defaultServerCmd
		if body.ServerCmd != "" {
			serverCmdStr = body.ServerCmd
		}

		// parse command string -> exec.Command args (simple split)
		parts := strings.Fields(serverCmdStr)
		if len(parts) == 0 {
			http.Error(w, "invalid server command", http.StatusInternalServerError)
			return
		}
		cmdName := parts[0]
		cmdArgs := parts[1:]

		// build command context with timeout
		ctx2, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx2, cmdName, cmdArgs...)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// pass environment; optionally override token for this request
		cmd.Env = os.Environ()
		if body.GithubPAT != "" {
			// Note: repo checks for GITHUB_PERSONAL_ACCESS_TOKEN env var
			cmd.Env = append(cmd.Env, "GITHUB_PERSONAL_ACCESS_TOKEN="+body.GithubPAT)
		}

		stdin, err := cmd.StdinPipe()
		if err != nil {
			http.Error(w, "failed to open stdin pipe: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// start child process
		if err := cmd.Start(); err != nil {
			http.Error(w, "failed to start subprocess: "+err.Error()+" stderr:"+stderr.String(), http.StatusInternalServerError)
			return
		}

		// write payload and close stdin (server typically reads request then exits)
		_, _ = stdin.Write(payload)
		_, _ = stdin.Write([]byte("\n"))
		_ = stdin.Close()

		// wait for completion or timeout
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case err := <-done:
			if err != nil {
				// include child's stderr for debugging
				http.Error(w, fmt.Sprintf("subprocess error: %v, stderr: %s", err, stderr.String()), http.StatusInternalServerError)
				return
			}
		case <-ctx2.Done():
			_ = cmd.Process.Kill()
			http.Error(w, "subprocess timed out", http.StatusGatewayTimeout)
			return
		}

		// success: proxy stdout as JSON to caller
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(w, &stdout)
	}
}

func main() {
	var addr string
	var defaultServerCmd string
	var timeoutSec int

	flag.StringVar(&addr, "addr", ":8080", "address to listen on")
	flag.StringVar(&defaultServerCmd, "server-cmd", "./github-mcp-server stdio", "command used to start the stdio MCP server (quoted string)")
	flag.IntVar(&timeoutSec, "timeout", 25, "timeout in seconds for each MCP request")
	flag.Parse()

	http.HandleFunc("/call", callHandler(defaultServerCmd, time.Duration(timeoutSec)*time.Second))
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("http -> mcp proxy listening on %s (server-cmd=%q)\n", addr, defaultServerCmd)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("listen failed: %v", err)
	}
}