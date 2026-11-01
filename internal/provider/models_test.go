package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func ollamaMM(t *testing.T, h http.HandlerFunc) ModelManager {
	t.Helper()
	p := newTest(t, "ollama", h, nil)
	return p.(ModelManager)
}

func TestListModels(t *testing.T) {
	mm := ollamaMM(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" || r.Method != "GET" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `{"models":[{"name":"llama3.2:latest","size":2019393189},{"name":"qwen2.5:7b","size":4683087332}]}`)
	})
	models, err := mm.ListModels(context.Background())
	if err != nil || len(models) != 2 || models[1].Name != "qwen2.5:7b" || models[0].Size != 2019393189 {
		t.Fatalf("%v %v", models, err)
	}
}

func TestPullModelStreamsProgress(t *testing.T) {
	mm := ollamaMM(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pull" {
			t.Errorf("path %s", r.URL.Path)
		}
		b := readBody(t, r)
		if b["model"] != "llama3.2" || b["stream"] != true {
			t.Errorf("body %v", b)
		}
		io.WriteString(w, `{"status":"pulling manifest"}`+"\n"+
			`{"status":"pulling abc","total":100,"completed":40}`+"\n\n"+
			`{"status":"pulling abc","total":100,"completed":100}`+"\n"+
			`{"status":"success"}`+"\n")
	})
	var got []PullProgress
	err := mm.PullModel(context.Background(), "llama3.2", func(p PullProgress) { got = append(got, p) })
	if err != nil || len(got) != 4 || got[1].Percent() != 40 || got[3].Status != "success" || got[0].Percent() != -1 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestPullModelErrors(t *testing.T) {
	cases := map[string]struct {
		h    http.HandlerFunc
		want string
	}{
		"error line": {func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"error":"pull model manifest: file does not exist"}`+"\n")
		}, "file does not exist"},
		"interrupted": {func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"status":"pulling abc","total":10,"completed":1}`+"\n")
		}, "interrupted"},
		"http error": {func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }, "http 500"},
	}
	for name, tc := range cases {
		err := ollamaMM(t, tc.h).PullModel(context.Background(), "x", nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", name, err, tc.want)
		}
	}
}

func TestModelInstalled(t *testing.T) {
	models := []ModelInfo{{Name: "llama3.2:latest"}, {Name: "qwen2.5:7b"}}
	for name, want := range map[string]bool{
		"llama3.2": true, "llama3.2:latest": true, " llama3.2 ": true,
		"qwen2.5:7b": true, "qwen2.5": false, "qwen2.5:14b": false, "mistral": false,
	} {
		if got := ModelInstalled(models, name); got != want {
			t.Errorf("ModelInstalled(%q) = %v, want %v", name, got, want)
		}
	}
}

type fakeMM struct {
	models []ModelInfo
	pulled []string
	err    error
}

func (f *fakeMM) ListModels(context.Context) ([]ModelInfo, error) { return f.models, f.err }
func (f *fakeMM) PullModel(_ context.Context, name string, _ func(PullProgress)) error {
	f.pulled = append(f.pulled, name)
	return nil
}

func TestEnsureModel(t *testing.T) {
	ctx := context.Background()
	f := &fakeMM{models: []ModelInfo{{Name: "llama3.2:latest"}}}
	if pulled, err := EnsureModel(ctx, f, "llama3.2", nil); pulled || err != nil || len(f.pulled) != 0 {
		t.Fatalf("installed model must not be pulled: %v %v %v", pulled, err, f.pulled)
	}
	if pulled, err := EnsureModel(ctx, f, " mistral ", nil); !pulled || err != nil || f.pulled[0] != "mistral" {
		t.Fatalf("missing model must be pulled: %v %v %v", pulled, err, f.pulled)
	}
	if _, err := EnsureModel(ctx, f, "  ", nil); err == nil {
		t.Fatal("empty name should fail")
	}
	f.err = errors.New("down")
	if _, err := EnsureModel(ctx, f, "x", nil); err == nil || len(f.pulled) != 1 {
		t.Fatalf("list error must abort: %v", err)
	}
}

func TestThrottle(t *testing.T) {
	var got []PullProgress
	fn := Throttle(func(p PullProgress) { got = append(got, p) })
	fn(PullProgress{Status: "a"})
	fn(PullProgress{Status: "a"}) // duplicate: dropped
	for i := int64(0); i <= 100; i++ {
		fn(PullProgress{Status: "dl", Total: 1000, Completed: i}) // 0..10%
	}
	if len(got) != 1+11 {
		t.Fatalf("want 12 forwarded updates, got %d", len(got))
	}
}
