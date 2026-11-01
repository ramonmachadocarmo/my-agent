package provider

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"my-agent/internal/config"
)

func TestOllamaSendsKeepAliveAndNumCtx(t *testing.T) {
	cases := []struct {
		name      string
		keepAlive string
		numCtx    int
		want      any // JSON-decoded keep_alive
		wantCtx   bool
	}{
		{"default is 30m", "", 0, "30m", false},
		{"custom duration", "2h", 0, "2h", false},
		{"seconds become a number", "600", 4096, float64(600), true},
		{"negative keeps forever", "-1", 0, float64(-1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen := 0
			p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
				seen++
				b := readBody(t, r)
				if b["keep_alive"] != tc.want {
					t.Errorf("keep_alive = %#v, want %#v", b["keep_alive"], tc.want)
				}
				opts, _ := b["options"].(map[string]any)
				if _, has := opts["num_ctx"]; has != tc.wantCtx {
					t.Errorf("num_ctx present=%v, want %v (%v)", has, tc.wantCtx, opts)
				}
				if tc.wantCtx && opts["num_ctx"] != float64(tc.numCtx) {
					t.Errorf("num_ctx = %v", opts["num_ctx"])
				}
				io.WriteString(w, `{"message":{"content":"ok"},"done":true}`+"\n")
			}, func(pc *config.ProviderConfig) { pc.KeepAlive, pc.NumCtx = tc.keepAlive, tc.numCtx })
			// both the streaming and the plain path must carry the settings
			if _, err := p.Chat(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if _, err := p.(Streamer).ChatStream(context.Background(), req, nil); err != nil {
				t.Fatal(err)
			}
			if seen != 2 {
				t.Fatalf("expected 2 requests, saw %d", seen)
			}
		})
	}
}

func TestOllamaWarmup(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		b := readBody(t, r)
		msgs, _ := b["messages"].([]any)
		if r.URL.Path != "/api/chat" || len(msgs) != 0 || b["model"] != "m" || b["keep_alive"] != "30m" || b["stream"] != false {
			t.Errorf("warm-up must be an empty chat that only loads the model: %s %v", r.URL.Path, b)
		}
		io.WriteString(w, `{"model":"m","message":{"role":"assistant","content":""},"done":true,"done_reason":"load"}`)
	}, nil)
	if err := p.(Warmer).Warmup(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRunningModels(t *testing.T) {
	p := newTest(t, "ollama", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ps" || r.Method != "GET" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `{"models":[
			{"name":"qwen2.5:14b","size":9500,"size_vram":9500,"context_length":4096,"expires_at":"2099-01-01T00:00:00Z"},
			{"name":"big:70b","size":40000,"size_vram":10000,"context_length":8192,"expires_at":"2099-01-01T00:00:00Z"}]}`)
	}, nil)
	got, err := p.(Runner).RunningModels(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	if got[0].OnCPU() || got[0].GPUPercent() != 100 || got[0].ContextLength != 4096 || got[0].ExpiresAt.Year() != 2099 {
		t.Errorf("fully loaded model misread: %+v", got[0])
	}
	if !got[1].OnCPU() || got[1].GPUPercent() != 25 {
		t.Errorf("offloaded model misread: %+v (gpu %d%%)", got[1], got[1].GPUPercent())
	}
}

func TestRunningModelEdgeCases(t *testing.T) {
	if (RunningModel{}).OnCPU() || (RunningModel{}).GPUPercent() != 0 {
		t.Error("zero-size model must not be reported as offloaded")
	}
	if !(RunningModel{Size: 100, SizeVRAM: 0}).OnCPU() {
		t.Error("model with nothing in VRAM runs on the CPU")
	}
	_ = time.Now
}
