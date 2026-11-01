package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"my-agent/internal/i18n"
)

// Streamer is implemented by providers that can deliver a reply piece by
// piece. onDelta is called for every new chunk of text; the full text is
// returned at the end.
type Streamer interface {
	ChatStream(ctx context.Context, req ChatRequest, onDelta func(string)) (string, error)
}

// ChatStream streams from p when it supports it. Otherwise it falls back to a
// regular Chat call and delivers the whole reply as a single delta.
func ChatStream(ctx context.Context, p Provider, req ChatRequest, onDelta func(string)) (string, error) {
	if s, ok := p.(Streamer); ok {
		return s.ChatStream(ctx, req, onDelta)
	}
	text, err := p.Chat(ctx, req)
	if err == nil && onDelta != nil && text != "" {
		onDelta(text)
	}
	return text, err
}

// idle is how long a stream may stay silent before it is aborted. Unlike a
// total timeout it never cuts a reply that is still flowing, but it does
// cover slow model loading before the first token.
func (b base) idle() time.Duration {
	if b.cfg.TimeoutSeconds > 0 {
		return time.Duration(b.cfg.TimeoutSeconds) * time.Second
	}
	return 120 * time.Second
}

// postStream POSTs in as JSON and calls onLine for every line of the response
// body. The request has no overall timeout, only an idle one. A rejected or
// rate-limited key is replaced by the next one before anything is read.
func (b base) postStream(ctx context.Context, url string, fixed map[string]string, in any, onLine func(line []byte) error) error {
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return b.withKeys(ctx, fixed, func(headers map[string]string) error {
		return b.streamOnce(ctx, url, headers, payload, onLine)
	})
}

func (b base) streamOnce(ctx context.Context, url string, headers map[string]string, payload []byte, onLine func(line []byte) error) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	idle := b.idle()
	timer := time.AfterFunc(idle, func() {
		cancel(i18n.Errorf("provider.err.idle", idle, b.name))
	})
	defer timer.Stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return b.streamErr(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return &HTTPError{b.name, resp.StatusCode, strings.TrimSpace(string(body))}
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		timer.Reset(idle)
		if line := bytes.TrimSpace(sc.Bytes()); len(line) > 0 {
			if err := onLine(line); err != nil {
				return err
			}
		}
	}
	if err := sc.Err(); err != nil {
		return b.streamErr(ctx, err)
	}
	return nil
}

// streamErr reports our idle-timeout reason instead of a bare "context canceled".
func (b base) streamErr(ctx context.Context, err error) error {
	if c := context.Cause(ctx); c != nil && !errors.Is(c, context.Canceled) && !errors.Is(c, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", b.name, c)
	}
	return fmt.Errorf("%s: %w", b.name, err)
}

// sseData returns the payload of a Server-Sent Events "data:" line.
func sseData(line []byte) ([]byte, bool) {
	rest, ok := bytes.CutPrefix(line, []byte("data:"))
	if !ok {
		return nil, false
	}
	rest = bytes.TrimSpace(rest)
	if len(rest) == 0 || string(rest) == "[DONE]" {
		return nil, false
	}
	return rest, true
}

func emit(onDelta func(string), s string) {
	if onDelta != nil && s != "" {
		onDelta(s)
	}
}
