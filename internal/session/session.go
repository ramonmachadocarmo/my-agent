// Package session stores conversations of the interactive agent on disk, so
// they can be resumed later.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"my-agent/internal/provider"
)

// Session is one conversation.
type Session struct {
	ID       string             `json:"id"`
	Title    string             `json:"title"`
	Dir      string             `json:"dir"` // working directory it belongs to
	Agent    string             `json:"agent"`
	Provider string             `json:"provider"`
	Model    string             `json:"model"`
	Created  time.Time          `json:"created"`
	Updated  time.Time          `json:"updated"`
	Usage    provider.Usage     `json:"usage"`
	Messages []provider.Message `json:"messages"`
}

// New starts an empty session.
func New(dir string) *Session {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	now := time.Now()
	return &Session{ID: now.Format("20060102-150405") + "-" + hex.EncodeToString(b), Dir: dir, Created: now, Updated: now}
}

// TitleFrom makes a title out of the first message.
func TitleFrom(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > 60 {
		r := []rune(s)
		s = string(r[:57]) + "..."
	}
	return s
}

// Store keeps sessions as JSON files in a directory.
type Store struct{ Dir string }

func (st Store) path(id string) string {
	return filepath.Join(st.Dir, filepath.Base(id)+".json")
}

// Save writes the session (atomically).
func (st Store) Save(s *Session) error {
	if len(s.Messages) == 0 {
		return nil // nothing worth keeping
	}
	if err := os.MkdirAll(st.Dir, 0o700); err != nil {
		return err
	}
	s.Updated = time.Now()
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := st.path(s.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, st.path(s.ID))
}

// Load reads a session.
func (st Store) Load(id string) (*Session, error) {
	data, err := os.ReadFile(st.path(id))
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Delete removes a session.
func (st Store) Delete(id string) error {
	err := os.Remove(st.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Summary is a session without its messages.
type Summary struct {
	ID, Title, Dir, Agent, Model string
	Updated                      time.Time
	Messages                     int
}

// List returns the sessions, newest first. With dir set, the sessions of
// that working directory come first.
func (st Store) List(dir string) ([]Summary, error) {
	entries, err := os.ReadDir(st.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Summary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := st.Load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, Summary{ID: s.ID, Title: s.Title, Dir: s.Dir, Agent: s.Agent, Model: s.Model, Updated: s.Updated, Messages: len(s.Messages)})
	}
	sort.Slice(out, func(i, j int) bool {
		if dir != "" && (out[i].Dir == dir) != (out[j].Dir == dir) {
			return out[i].Dir == dir
		}
		return out[i].Updated.After(out[j].Updated)
	})
	return out, nil
}
