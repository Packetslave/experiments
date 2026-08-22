// Package server exposes the omnifocus.Client operation set as a thin REST
// API plus the embedded phone frontend. It is the HTTP twin of the TUI: the
// phone fetches the inbox into a local queue and processes it card by card.
package server

import (
	"embed"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
)

//go:embed assets
var assets embed.FS

// Server serializes all Client calls behind a mutex: the osascript backend
// must never run two Apple Event conversations at once.
type Server struct {
	mu     sync.Mutex
	client omnifocus.Client
}

// New returns the http.Handler serving the API and frontend.
func New(client omnifocus.Client) http.Handler {
	s := &Server{client: client}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/inbox", s.handleInbox)
	mux.HandleFunc("GET /api/projects", s.handleProjects)
	mux.HandleFunc("GET /api/tags", s.handleTags)
	mux.HandleFunc("POST /api/tasks/{id}/complete", s.taskAction(func(r *http.Request, id string) error {
		return s.client.Complete(r.Context(), id)
	}))
	mux.HandleFunc("POST /api/tasks/{id}/drop", s.taskAction(func(r *http.Request, id string) error {
		return s.client.Drop(r.Context(), id)
	}))
	mux.HandleFunc("POST /api/tasks/{id}/move", s.taskAction(func(r *http.Request, id string) error {
		var body struct {
			ProjectID string `json:"projectId"`
		}
		if err := decodeBody(r, &body); err != nil || body.ProjectID == "" {
			return errBadRequest
		}
		return s.client.MoveToProject(r.Context(), id, body.ProjectID)
	}))
	mux.HandleFunc("POST /api/tasks/{id}/tag", s.taskAction(func(r *http.Request, id string) error {
		var body struct {
			TagID string `json:"tagId"`
		}
		if err := decodeBody(r, &body); err != nil || body.TagID == "" {
			return errBadRequest
		}
		return s.client.AddTag(r.Context(), id, body.TagID)
	}))
	mux.HandleFunc("POST /api/tasks/{id}/flag", s.taskAction(func(r *http.Request, id string) error {
		var body struct {
			Flagged *bool `json:"flagged"`
		}
		if err := decodeBody(r, &body); err != nil || body.Flagged == nil {
			return errBadRequest
		}
		return s.client.SetFlagged(r.Context(), id, *body.Flagged)
	}))
	mux.HandleFunc("POST /api/tasks/{id}/defer", s.taskAction(func(r *http.Request, id string) error {
		date, err := decodeDateBody(r)
		if err != nil {
			return err
		}
		return s.client.SetDeferDate(r.Context(), id, date)
	}))
	mux.HandleFunc("POST /api/tasks/{id}/due", s.taskAction(func(r *http.Request, id string) error {
		date, err := decodeDateBody(r)
		if err != nil {
			return err
		}
		return s.client.SetDueDate(r.Context(), id, date)
	}))
	mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	mux.HandleFunc("POST /api/tags", s.handleCreateTag)
	mux.HandleFunc("GET /{$}", serveAsset("assets/index.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /manifest.webmanifest", serveAsset("assets/manifest.webmanifest", "application/manifest+json"))
	mux.HandleFunc("GET /apple-touch-icon.png", serveAsset("assets/apple-touch-icon.png", "image/png"))
	return mux
}

func serveAsset(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := assets.ReadFile(name)
		if err != nil {
			http.Error(w, "asset missing", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}
}

func decodeName(r *http.Request) (string, error) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeBody(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		return "", errBadRequest
	}
	return strings.TrimSpace(body.Name), nil
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	name, err := decodeName(r)
	if err != nil {
		writeError(w, err)
		return
	}
	s.mu.Lock()
	p, err := s.client.CreateProject(r.Context(), name)
	s.mu.Unlock()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, projectJSON(p))
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	name, err := decodeName(r)
	if err != nil {
		writeError(w, err)
		return
	}
	s.mu.Lock()
	g, err := s.client.CreateTag(r.Context(), name)
	s.mu.Unlock()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tagJSON(g))
}

// errBadRequest marks a malformed request body; writeError maps it to 400.
var errBadRequest = errors.New("bad request")

func decodeBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// decodeDateBody parses {"date": "<RFC3339>"} bodies; a null or "" date
// means clear and returns nil.
func decodeDateBody(r *http.Request) (*time.Time, error) {
	var body struct {
		Date *string `json:"date"`
	}
	if err := decodeBody(r, &body); err != nil {
		return nil, errBadRequest
	}
	if body.Date == nil || *body.Date == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *body.Date)
	if err != nil {
		return nil, errBadRequest
	}
	local := t.Local()
	return &local, nil
}

// taskAction wraps a mutation on the task named by the {id} path segment,
// holding the client mutex and mapping errors to statuses.
func (s *Server) taskAction(fn func(r *http.Request, id string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		err := fn(r, r.PathValue("id"))
		s.mu.Unlock()
		if err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type taskJSON struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Note       string   `json:"note"`
	Flagged    bool     `json:"flagged"`
	DeferDate  *string  `json:"deferDate"`
	DueDate    *string  `json:"dueDate"`
	Tags       []string `json:"tags"`
	ChildCount int      `json:"childCount"`
	LinkURL    string   `json:"linkUrl"`
}

func isoPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func toTaskJSON(t omnifocus.Task) taskJSON {
	url, _ := t.LinkURL()
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	return taskJSON{
		ID:         t.ID,
		Name:       t.Name,
		Note:       t.Note,
		Flagged:    t.Flagged,
		DeferDate:  isoPtr(t.DeferDate),
		DueDate:    isoPtr(t.DueDate),
		Tags:       tags,
		ChildCount: t.ChildCount,
		LinkURL:    url,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	tasks, err := s.client.InboxTasks(r.Context())
	s.mu.Unlock()
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]taskJSON, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toTaskJSON(t))
	}
	writeJSON(w, http.StatusOK, out)
}

type projectJSON struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
	Status string `json:"status"`
}

type tagJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	projects, err := s.client.Projects(r.Context())
	s.mu.Unlock()
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]projectJSON, 0, len(projects))
	for _, p := range projects {
		out = append(out, projectJSON(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	tags, err := s.client.Tags(r.Context())
	s.mu.Unlock()
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]tagJSON, 0, len(tags))
	for _, g := range tags {
		out = append(out, tagJSON(g))
	}
	writeJSON(w, http.StatusOK, out)
}

// writeError maps client errors onto the API's error contract: malformed
// bodies are 400, a task that no longer exists is 410 Gone (both backends
// phrase this as "task not found: …" — the frontend advances past the card
// on 410, so only task lookups may map here; a vanished project or tag is a
// 502 the user must see), and anything else — an unreachable OmniFocus, a
// failed osascript — is 502.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, errBadRequest):
		status = http.StatusBadRequest
	case strings.Contains(err.Error(), "task not found"):
		status = http.StatusGone
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
