package instapaper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Authenticate_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1/oauth/access_token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		_ = r.ParseForm()
		if r.FormValue("x_auth_mode") != "client_auth" {
			t.Errorf("x_auth_mode: got %q", r.FormValue("x_auth_mode"))
		}
		if r.FormValue("oauth_consumer_key") == "" {
			t.Error("oauth_consumer_key missing")
		}
		if r.FormValue("oauth_signature") == "" {
			t.Error("oauth_signature missing")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("oauth_token=testtoken&oauth_token_secret=testsecret"))
	}))
	defer srv.Close()

	c := NewClient("ckey", "csecret", "user@example.com", "pass")
	c.baseURL = srv.URL

	if err := c.Authenticate(); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if c.accessToken != "testtoken" {
		t.Errorf("accessToken: got %q, want testtoken", c.accessToken)
	}
	if c.accessSecret != "testsecret" {
		t.Errorf("accessSecret: got %q, want testsecret", c.accessSecret)
	}
}

func TestClient_Authenticate_non_200_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if err := c.Authenticate(); err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
}

func TestClient_Add_returns_bookmark_id(t *testing.T) {
	type bm struct {
		BookmarkID int64 `json:"bookmark_id"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1.1/bookmarks/add" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.FormValue("url") == "" {
			t.Error("url param missing")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]bm{{BookmarkID: 99999}})
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL
	c.accessToken = "tok"
	c.accessSecret = "toksecret"

	id, err := c.Add("https://example.com/article", "Title")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != 99999 {
		t.Errorf("bookmark_id: got %d, want 99999", id)
	}
}

func TestClient_Add_empty_title_omits_param(t *testing.T) {
	type bm struct {
		BookmarkID int64 `json:"bookmark_id"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("title") != "" {
			t.Errorf("expected no title param, got %q", r.FormValue("title"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]bm{{BookmarkID: 1}})
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if _, err := c.Add("https://example.com/article", ""); err != nil {
		t.Fatalf("Add empty title: %v", err)
	}
}

func TestClient_Add_non_200_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	_, err := c.Add("https://example.com/article", "Title")
	if err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
}

func TestClient_Archive_sends_bookmark_id(t *testing.T) {
	var gotBookmarkID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1.1/bookmarks/archive" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = r.ParseForm()
		gotBookmarkID = r.FormValue("bookmark_id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{{"bookmark_id": 42}})
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if err := c.Archive(42); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if gotBookmarkID != "42" {
		t.Errorf("bookmark_id: got %q, want \"42\"", gotBookmarkID)
	}
}

func TestClient_Archive_non_200_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if err := c.Archive(1); err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
}
