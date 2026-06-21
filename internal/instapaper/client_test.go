package instapaper

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClient_Add_success(t *testing.T) {
	var gotUsername, gotPassword, gotURL, gotTitle string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotUsername = r.FormValue("username")
		gotPassword = r.FormValue("password")
		gotURL = r.FormValue("url")
		gotTitle = r.FormValue("title")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("user@example.com", "secret")
	c.baseURL = srv.URL

	err := c.Add("https://example.com/article", "Article Title")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if gotUsername != "user@example.com" {
		t.Errorf("username: got %q", gotUsername)
	}
	if gotPassword != "secret" {
		t.Errorf("password: got %q", gotPassword)
	}
	if gotURL != "https://example.com/article" {
		t.Errorf("url: got %q", gotURL)
	}
	if gotTitle != "Article Title" {
		t.Errorf("title: got %q", gotTitle)
	}
}

func TestClient_Add_empty_title(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("title") != "" {
			t.Errorf("expected no title param, got %q", r.FormValue("title"))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("u", "p")
	c.baseURL = srv.URL

	if err := c.Add("https://example.com/article", ""); err != nil {
		t.Fatalf("Add with empty title: %v", err)
	}
}

func TestClient_Add_non_201_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient("u", "p")
	c.baseURL = srv.URL

	err := c.Add("https://example.com/article", "Title")
	if err == nil {
		t.Fatal("expected error for non-201 response, got nil")
	}
}

func TestClient_Add_encodes_url_in_form(t *testing.T) {
	var rawBody url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		rawBody = r.Form
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("u", "p")
	c.baseURL = srv.URL

	articleURL := "https://example.com/path?q=hello world&a=1"
	_ = c.Add(articleURL, "")

	if rawBody.Get("url") != articleURL {
		t.Errorf("url not properly form-encoded: got %q", rawBody.Get("url"))
	}
}
