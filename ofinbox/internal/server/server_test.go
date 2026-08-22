package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
	"github.com/packetslave/experiments/ofinbox/internal/server"
)

func newTestServer(t *testing.T) (*httptest.Server, *omnifocus.DemoClient) {
	t.Helper()
	client := omnifocus.NewDemoClient()
	ts := httptest.NewServer(server.New(client))
	t.Cleanup(ts.Close)
	return ts, client
}

func getJSON(t *testing.T, ts *httptest.Server, path string, out any) {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("GET %s: decoding: %v", path, err)
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

func postJSON(t *testing.T, ts *httptest.Server, path, body string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	resp, err := http.Post(ts.URL+path, "application/json", rdr)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func inboxTask(t *testing.T, ts *httptest.Server, id string) (taskJSON, bool) {
	t.Helper()
	var tasks []taskJSON
	getJSON(t, ts, "/api/inbox", &tasks)
	for _, task := range tasks {
		if task.ID == id {
			return task, true
		}
	}
	return taskJSON{}, false
}

func TestCompleteRemovesTaskFromInbox(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := postJSON(t, ts, "/api/tasks/t1/complete", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("complete: status %d, want 204", resp.StatusCode)
	}
	if _, ok := inboxTask(t, ts, "t1"); ok {
		t.Error("t1 still in inbox after complete")
	}
}

func TestMutatingVanishedTaskReturns410(t *testing.T) {
	ts, _ := newTestServer(t)

	postJSON(t, ts, "/api/tasks/t1/complete", "")
	for path, body := range map[string]string{
		"/api/tasks/t1/complete": "",
		"/api/tasks/t1/drop":     "",
		"/api/tasks/t1/move":     `{"projectId":"p1"}`,
		"/api/tasks/t1/tag":      `{"tagId":"g1"}`,
		"/api/tasks/t1/flag":     `{"flagged":true}`,
		"/api/tasks/t1/defer":    `{"date":"2026-09-01T08:00:00Z"}`,
		"/api/tasks/t1/due":      `{"date":"2026-09-01T17:00:00Z"}`,
	} {
		if resp := postJSON(t, ts, path, body); resp.StatusCode != http.StatusGone {
			t.Errorf("POST %s on vanished task: status %d, want 410", path, resp.StatusCode)
		}
	}
}

func TestVanishedProjectOrTagIsNot410(t *testing.T) {
	ts, _ := newTestServer(t)

	// 410 tells the frontend "this task is gone, advance past it" — a
	// vanished filing target must not be mistaken for that.
	for path, body := range map[string]string{
		"/api/tasks/t1/move": `{"projectId":"p-gone"}`,
		"/api/tasks/t1/tag":  `{"tagId":"g-gone"}`,
	} {
		if resp := postJSON(t, ts, path, body); resp.StatusCode != http.StatusBadGateway {
			t.Errorf("POST %s with vanished target: status %d, want 502", path, resp.StatusCode)
		}
	}
	if _, ok := inboxTask(t, ts, "t1"); !ok {
		t.Error("t1 gone after failed filing attempts")
	}
}

func TestDropRemovesTaskFromInbox(t *testing.T) {
	ts, _ := newTestServer(t)

	if resp := postJSON(t, ts, "/api/tasks/t2/drop", ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("drop: status %d, want 204", resp.StatusCode)
	}
	if _, ok := inboxTask(t, ts, "t2"); ok {
		t.Error("t2 still in inbox after drop")
	}
}

func TestMoveFilesTaskToProject(t *testing.T) {
	ts, _ := newTestServer(t)

	if resp := postJSON(t, ts, "/api/tasks/t1/move", `{"projectId":"p2"}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("move: status %d, want 204", resp.StatusCode)
	}
	if _, ok := inboxTask(t, ts, "t1"); ok {
		t.Error("t1 still in inbox after filing")
	}
}

func TestMalformedBodyReturns400(t *testing.T) {
	ts, _ := newTestServer(t)

	for path, body := range map[string]string{
		"/api/tasks/t1/move":  `{`,
		"/api/tasks/t1/tag":   `{}`,
		"/api/tasks/t1/flag":  `{}`,
		"/api/tasks/t1/defer": `{"date":"not-a-date"}`,
		"/api/tasks/t1/due":   `{"date":"not-a-date"}`,
	} {
		if resp := postJSON(t, ts, path, body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("POST %s with body %q: status %d, want 400", path, body, resp.StatusCode)
		}
	}
	// Malformed requests must not have touched the task.
	if _, ok := inboxTask(t, ts, "t1"); !ok {
		t.Error("t1 gone after malformed requests")
	}
}

func TestTagFlagAndDatesUpdateTask(t *testing.T) {
	ts, _ := newTestServer(t)

	if resp := postJSON(t, ts, "/api/tasks/t1/tag", `{"tagId":"g1"}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("tag: status %d, want 204", resp.StatusCode)
	}
	if resp := postJSON(t, ts, "/api/tasks/t1/flag", `{"flagged":true}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("flag: status %d, want 204", resp.StatusCode)
	}
	if resp := postJSON(t, ts, "/api/tasks/t1/defer", `{"date":"2026-09-01T08:00:00Z"}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("defer: status %d, want 204", resp.StatusCode)
	}

	task, ok := inboxTask(t, ts, "t1")
	if !ok {
		t.Fatal("t1 missing from inbox")
	}
	if len(task.Tags) != 1 || task.Tags[0] != "errand" {
		t.Errorf("t1 tags = %v, want [errand]", task.Tags)
	}
	if !task.Flagged {
		t.Error("t1 not flagged")
	}
	if task.DeferDate == nil {
		t.Fatal("t1 deferDate is null after set")
	}

	// null date clears.
	if resp := postJSON(t, ts, "/api/tasks/t1/defer", `{"date":null}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("clear defer: status %d, want 204", resp.StatusCode)
	}
	task, _ = inboxTask(t, ts, "t1")
	if task.DeferDate != nil {
		t.Errorf("t1 deferDate = %q after clear, want null", *task.DeferDate)
	}

	// due works the same way.
	if resp := postJSON(t, ts, "/api/tasks/t1/due", `{"date":"2026-09-02T17:00:00Z"}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("due: status %d, want 204", resp.StatusCode)
	}
	task, _ = inboxTask(t, ts, "t1")
	if task.DueDate == nil {
		t.Error("t1 dueDate is null after set")
	}
}

func TestProjectsAndTagsAreListed(t *testing.T) {
	ts, _ := newTestServer(t)

	var projects []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Folder string `json:"folder"`
		Status string `json:"status"`
	}
	getJSON(t, ts, "/api/projects", &projects)
	found := false
	for _, p := range projects {
		if p.Name == "Links to Review" && p.Folder == "Personal" && p.Status == "active" {
			found = true
		}
	}
	if !found {
		t.Error(`demo project "Links to Review" (Personal, active) missing from /api/projects`)
	}

	var tags []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	getJSON(t, ts, "/api/tags", &tags)
	found = false
	for _, g := range tags {
		if g.Name == "NoAction" && g.ID != "" {
			found = true
		}
	}
	if !found {
		t.Error(`demo tag "NoAction" missing from /api/tags`)
	}
}

func TestFrontendIsServed(t *testing.T) {
	ts, _ := newTestServer(t)

	for path, contentType := range map[string]string{
		"/":                     "text/html",
		"/manifest.webmanifest": "application/manifest+json",
		"/apple-touch-icon.png": "image/png",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", path, resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, contentType) {
			t.Errorf("GET %s: Content-Type %q, want %q prefix", path, got, contentType)
		}
		if path == "/" && !strings.Contains(string(body), "<title>Inbox</title>") {
			t.Error("GET /: response is not the Inbox app")
		}
	}

	// Unknown paths must not fall through to the app shell.
	resp, err := http.Get(ts.URL + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /nope: status %d, want 404", resp.StatusCode)
	}
}

func TestCreateProjectThenFileToIt(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := postJSON(t, ts, "/api/projects", `{"name":"Garage cleanout"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project: status %d, want 201", resp.StatusCode)
	}
	var made struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&made); err != nil {
		t.Fatalf("decoding created project: %v", err)
	}
	if made.ID == "" || made.Name != "Garage cleanout" {
		t.Fatalf("created project = %+v, want assigned ID and given name", made)
	}

	if resp := postJSON(t, ts, "/api/tasks/t1/move", `{"projectId":"`+made.ID+`"}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("move to new project: status %d, want 204", resp.StatusCode)
	}
	if _, ok := inboxTask(t, ts, "t1"); ok {
		t.Error("t1 still in inbox after filing to new project")
	}
}

func TestCreateTagThenTagWithIt(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := postJSON(t, ts, "/api/tags", `{"name":"someday"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create tag: status %d, want 201", resp.StatusCode)
	}
	var made struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&made); err != nil {
		t.Fatalf("decoding created tag: %v", err)
	}

	if resp := postJSON(t, ts, "/api/tasks/t1/tag", `{"tagId":"`+made.ID+`"}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("tag with new tag: status %d, want 204", resp.StatusCode)
	}
	task, _ := inboxTask(t, ts, "t1")
	if len(task.Tags) != 1 || task.Tags[0] != "someday" {
		t.Errorf("t1 tags = %v, want [someday]", task.Tags)
	}
}

func TestCreateWithEmptyNameReturns400(t *testing.T) {
	ts, _ := newTestServer(t)

	for _, path := range []string{"/api/projects", "/api/tags"} {
		if resp := postJSON(t, ts, path, `{"name":""}`); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("POST %s with empty name: status %d, want 400", path, resp.StatusCode)
		}
	}
}

func TestInboxReturnsTopLevelTasksWithLinkDetection(t *testing.T) {
	ts, _ := newTestServer(t)

	var tasks []taskJSON
	getJSON(t, ts, "/api/inbox", &tasks)

	if len(tasks) == 0 {
		t.Fatal("inbox is empty, want demo tasks")
	}
	byID := map[string]taskJSON{}
	for _, task := range tasks {
		byID[task.ID] = task
	}

	// t7 is an action group in the demo data; its children are not inbox items.
	if _, ok := byID["t7a"]; ok {
		t.Error("child task t7a appeared in the inbox")
	}
	if got := byID["t7"].ChildCount; got != 3 {
		t.Errorf("t7 childCount = %d, want 3", got)
	}

	// t9 is a URL-only title; t1 is prose.
	if got := byID["t9"].LinkURL; got != "https://go.dev/blog/loopvar-preview" {
		t.Errorf("t9 linkUrl = %q, want the URL", got)
	}
	if got := byID["t1"].LinkURL; got != "" {
		t.Errorf("t1 linkUrl = %q, want empty", got)
	}

	// t3 has a due date; dates are RFC3339 strings or null.
	if byID["t3"].DueDate == nil {
		t.Error("t3 dueDate is null, want RFC3339 string")
	}
	if byID["t1"].DueDate != nil {
		t.Errorf("t1 dueDate = %q, want null", *byID["t1"].DueDate)
	}
}
