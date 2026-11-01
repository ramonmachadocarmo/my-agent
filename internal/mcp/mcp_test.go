package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"my-agent/internal/config"
)

// A fake server: the test binary serves MCP over stdio when this variable is
// set, so the stdio transport is tested against a real subprocess.
const fakeEnv = "MY_AGENT_FAKE_MCP"

func TestMain(m *testing.M) {
	if os.Getenv(fakeEnv) == "1" {
		serveFake(os.Stdin, os.Stdout)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// handle answers one JSON-RPC request; notifications get no answer.
func handle(line []byte) []byte {
	var req struct {
		ID     *json.RawMessage `json:"id"`
		Method string           `json:"method"`
		Params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"params"`
	}
	if json.Unmarshal(line, &req) != nil || req.ID == nil {
		return nil
	}
	var result any
	switch req.Method {
	case "initialize":
		result = map[string]any{"protocolVersion": ProtocolVersion, "serverInfo": map[string]any{"name": "fake", "version": "1.0"}, "capabilities": map[string]any{"tools": map[string]any{}}}
	case "tools/list":
		result = map[string]any{"tools": []any{map[string]any{"name": "echo", "description": "echoes", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}}}}}
	case "tools/call":
		if req.Params.Name == "fail" {
			result = map[string]any{"isError": true, "content": []any{map[string]any{"type": "text", "text": "boom"}}}
		} else {
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": fmt.Sprint("echo: ", req.Params.Arguments["text"])}}}
		}
	default:
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "no"}})
		return b
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	return b
}

func serveFake(in io.Reader, out io.Writer) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		if resp := handle(sc.Bytes()); resp != nil {
			fmt.Fprintf(out, "%s\n", resp)
		}
	}
}

func checkClient(t *testing.T, c *Client) {
	t.Helper()
	if len(c.Tools) != 1 || c.Tools[0].Name != "echo" || c.Server != "fake 1.0" {
		t.Fatalf("client: %+v", c)
	}
	text, isErr, err := c.Call(context.Background(), "echo", map[string]any{"text": "hi"})
	if err != nil || isErr || text != "echo: hi" {
		t.Fatalf("call: %q %v %v", text, isErr, err)
	}
	text, isErr, err = c.Call(context.Background(), "fail", nil)
	if err != nil || !isErr || text != "boom" {
		t.Fatalf("failing call: %q %v %v", text, isErr, err)
	}
}

func TestStdioServer(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.MCPConfig{Type: "local", Command: []string{exe, "-test.run=^$"}, Environment: map[string]string{fakeEnv: "1"}}
	c, err := Connect(context.Background(), "fake", cfg, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	checkClient(t, c)
}

func TestStreamableHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing header: %v", r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		resp := handle(body)
		w.Header().Set("Mcp-Session-Id", "s1")
		if resp == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if strings.Contains(string(body), "tools/call") { // answer this one as an event stream
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", resp)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}))
	defer srv.Close()
	cfg := config.MCPConfig{Type: "remote", URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer tok"}}
	c, err := Connect(context.Background(), "web", cfg, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	checkClient(t, c)
}

func TestManagerToolsAndNames(t *testing.T) {
	if got := ToolName("git hub", "search/repos"); got != "mcp_git_hub_search_repos" {
		t.Fatalf("name %q", got)
	}
	exe, _ := os.Executable()
	m := NewManager()
	m.Start(context.Background(), map[string]config.MCPConfig{
		"fake": {Type: "local", Command: []string{exe, "-test.run=^$"}, Environment: map[string]string{fakeEnv: "1"}},
		"off":  {Type: "local", Command: []string{"nothing"}, Enabled: new(bool)},
		"bad":  {Type: "local", Command: []string{"this-command-does-not-exist-xyz"}},
	}, "test")
	m.Wait()
	defer m.Close()
	st := m.Statuses()
	if len(st) != 3 || st[0].Name != "bad" || st[0].Err == nil || !st[2].Disabled || st[1].Tools != 1 {
		t.Fatalf("statuses: %+v", st)
	}
	ts := m.Tools()
	if len(ts) != 1 || ts[0].Name != "mcp_fake_echo" {
		t.Fatalf("tools: %+v", ts)
	}
	res, err := ts[0].Run(context.Background(), nil, map[string]any{"text": "x"})
	if err != nil || res.Output != "echo: x" {
		t.Fatalf("%+v %v", res, err)
	}
}
