// Package mcp is a small Model Context Protocol client: it starts local
// servers (stdio) or connects to remote ones (Streamable HTTP), lists their
// tools and calls them.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"my-agent/internal/config"
)

// ProtocolVersion is the MCP revision this client speaks.
const ProtocolVersion = "2025-06-18"

// ToolInfo is a tool offered by a server.
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// RPCError is a JSON-RPC error returned by a server.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message) }

type transport interface {
	call(ctx context.Context, method string, params any) (json.RawMessage, error)
	notify(ctx context.Context, method string, params any) error
	close() error
}

// Client is a connection to one server.
type Client struct {
	Name   string
	Tools  []ToolInfo
	Server string // server name and version reported by initialize
	t      transport
}

// Connect starts or reaches the server, runs the handshake and lists its
// tools.
func Connect(ctx context.Context, name string, cfg config.MCPConfig, version string) (*Client, error) {
	timeout := 30 * time.Second
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	var t transport
	var err error
	switch cfg.Type {
	case "local":
		t, err = startStdio(name, cfg)
	case "remote":
		t = &httpTransport{url: cfg.URL, headers: cfg.Headers, client: &http.Client{}}
	default:
		err = fmt.Errorf("mcp %s: unknown type %q", name, cfg.Type)
	}
	if err != nil {
		return nil, err
	}
	c := &Client{Name: name, t: t}
	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := c.handshake(hctx, version); err != nil {
		t.close()
		return nil, err
	}
	return c, nil
}

func (c *Client) handshake(ctx context.Context, version string) error {
	raw, err := c.t.call(ctx, "initialize", map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "my-agent", "version": version},
	})
	if err != nil {
		return err
	}
	var init struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	_ = json.Unmarshal(raw, &init)
	c.Server = strings.TrimSpace(init.ServerInfo.Name + " " + init.ServerInfo.Version)
	if err := c.t.notify(ctx, "notifications/initialized", nil); err != nil {
		return err
	}
	cursor := ""
	for page := 0; page < 20; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.t.call(ctx, "tools/list", params)
		if err != nil {
			return err
		}
		var res struct {
			Tools      []ToolInfo `json:"tools"`
			NextCursor string     `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return fmt.Errorf("mcp %s: tools/list: %w", c.Name, err)
		}
		c.Tools = append(c.Tools, res.Tools...)
		if cursor = res.NextCursor; cursor == "" {
			break
		}
	}
	return nil
}

// Call runs a tool and returns its text content. isError reports a tool
// level failure (the call itself worked).
func (c *Client) Call(ctx context.Context, tool string, args map[string]any) (text string, isError bool, err error) {
	if args == nil {
		args = map[string]any{}
	}
	raw, err := c.t.call(ctx, "tools/call", map[string]any{"name": tool, "arguments": args})
	if err != nil {
		return "", false, err
	}
	var res struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			MimeType string `json:"mimeType"`
			Resource struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"resource"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		IsError           bool            `json:"isError"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", false, err
	}
	var sb strings.Builder
	for _, part := range res.Content {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		switch part.Type {
		case "text":
			sb.WriteString(part.Text)
		case "resource":
			sb.WriteString(part.Resource.URI + "\n" + part.Resource.Text)
		default:
			fmt.Fprintf(&sb, "[%s %s]", part.Type, part.MimeType)
		}
	}
	if sb.Len() == 0 && len(res.StructuredContent) > 0 {
		sb.Write(res.StructuredContent)
	}
	return sb.String(), res.IsError, nil
}

// Close stops the server (local) or ends the session (remote).
func (c *Client) Close() error { return c.t.close() }

// ---- stdio ----

type stdioTransport struct {
	name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	nextID atomic.Int64

	mu      sync.Mutex
	pending map[int64]chan rpcResponse
	done    chan struct{}
	stderr  *tailBuffer
	werr    sync.Mutex
}

type rpcResponse struct {
	ID     *json.RawMessage `json:"id"`
	Method string           `json:"method"`
	Result json.RawMessage  `json:"result"`
	Error  *RPCError        `json:"error"`
}

func startStdio(name string, cfg config.MCPConfig) (*stdioTransport, error) {
	cmd := exec.Command(cfg.Command[0], cfg.Command[1:]...)
	cmd.Env = os.Environ()
	for k, v := range cfg.Environment {
		cmd.Env = append(cmd.Env, k+"="+os.ExpandEnv(v))
	}
	hideWindow(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	t := &stdioTransport{name: name, cmd: cmd, stdin: stdin, pending: map[int64]chan rpcResponse{}, done: make(chan struct{}), stderr: &tailBuffer{max: 4096}}
	cmd.Stderr = t.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp %s: %w", name, err)
	}
	go t.read(stdout)
	return t, nil
}

func (t *stdioTransport) read(r io.Reader) {
	defer close(t.done)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 32<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var msg rpcResponse
		if json.Unmarshal(line, &msg) != nil || msg.ID == nil {
			continue // notification
		}
		if msg.Method != "" { // a request from the server (ping, roots/list...)
			t.reply(*msg.ID, msg.Method)
			continue
		}
		var id int64
		if json.Unmarshal(*msg.ID, &id) != nil {
			continue
		}
		t.mu.Lock()
		ch := t.pending[id]
		delete(t.pending, id)
		t.mu.Unlock()
		if ch != nil {
			ch <- msg
		}
	}
}

func (t *stdioTransport) reply(id json.RawMessage, method string) {
	msg := map[string]any{"jsonrpc": "2.0", "id": id}
	if method == "ping" {
		msg["result"] = map[string]any{}
	} else {
		msg["error"] = map[string]any{"code": -32601, "message": "method not supported"}
	}
	_ = t.write(msg)
}

func (t *stdioTransport) write(msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	t.werr.Lock()
	defer t.werr.Unlock()
	_, err = t.stdin.Write(append(b, '\n'))
	return err
}

func (t *stdioTransport) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := t.nextID.Add(1)
	ch := make(chan rpcResponse, 1)
	t.mu.Lock()
	t.pending[id] = ch
	t.mu.Unlock()
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if err := t.write(msg); err != nil {
		return nil, t.exitErr(err)
	}
	select {
	case r := <-ch:
		if r.Error != nil {
			return nil, r.Error
		}
		return r.Result, nil
	case <-t.done:
		return nil, t.exitErr(errors.New("server exited"))
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		_ = t.write(map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled", "params": map[string]any{"requestId": id}})
		return nil, ctx.Err()
	}
}

func (t *stdioTransport) exitErr(err error) error {
	if s := strings.TrimSpace(t.stderr.String()); s != "" {
		return fmt.Errorf("mcp %s: %w: %s", t.name, err, s)
	}
	return fmt.Errorf("mcp %s: %w", t.name, err)
}

func (t *stdioTransport) notify(_ context.Context, method string, params any) error {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	return t.write(msg)
}

func (t *stdioTransport) close() error {
	t.stdin.Close()
	select {
	case <-t.done:
	case <-time.After(2 * time.Second):
	}
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	_ = t.cmd.Wait()
	return nil
}

// tailBuffer keeps the last max bytes written (a server's stderr).
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = b.buf[len(b.buf)-b.max:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// ---- Streamable HTTP ----

type httpTransport struct {
	url     string
	headers map[string]string
	client  *http.Client
	nextID  atomic.Int64
	session atomic.Value // string
}

func (t *httpTransport) post(ctx context.Context, msg map[string]any) (*http.Response, error) {
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	if s, _ := t.session.Load().(string); s != "" {
		req.Header.Set("Mcp-Session-Id", s)
	}
	for k, v := range t.headers {
		req.Header.Set(k, os.ExpandEnv(v))
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	if s := resp.Header.Get("Mcp-Session-Id"); s != "" {
		t.session.Store(s)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		resp.Body.Close()
		return nil, fmt.Errorf("mcp %s: http %d: %s", t.url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func (t *httpTransport) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := t.nextID.Add(1)
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	resp, err := t.post(ctx, msg)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	match := func(data []byte) (json.RawMessage, bool, error) {
		var r rpcResponse
		if json.Unmarshal(data, &r) != nil || r.ID == nil || r.Method != "" {
			return nil, false, nil
		}
		var got int64
		if json.Unmarshal(*r.ID, &got) != nil || got != id {
			return nil, false, nil
		}
		if r.Error != nil {
			return nil, true, r.Error
		}
		return r.Result, true, nil
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 64<<10), 32<<20)
		var data bytes.Buffer
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 { // end of event
				if res, ok, err := match(data.Bytes()); ok {
					return res, err
				}
				data.Reset()
				continue
			}
			if rest, ok := bytes.CutPrefix(line, []byte("data:")); ok {
				data.Write(bytes.TrimSpace(rest))
			}
		}
		if res, ok, err := match(data.Bytes()); ok {
			return res, err
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("mcp %s: no response to %s", t.url, method)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	res, ok, err := match(body)
	if !ok && err == nil {
		return nil, fmt.Errorf("mcp %s: unexpected response to %s", t.url, method)
	}
	return res, err
}

func (t *httpTransport) notify(ctx context.Context, method string, params any) error {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	resp, err := t.post(ctx, msg)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (t *httpTransport) close() error {
	s, _ := t.session.Load().(string)
	if s == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, t.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Mcp-Session-Id", s)
	if resp, err := t.client.Do(req); err == nil {
		resp.Body.Close()
	}
	return nil
}
