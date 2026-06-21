package instapaper

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	consumerKey    string
	consumerSecret string
	username       string
	password       string
	accessToken    string
	accessSecret   string
	baseURL        string
	http           *http.Client
}

func NewClient(consumerKey, consumerSecret, username, password string) *Client {
	return &Client{
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		username:       username,
		password:       password,
		baseURL:        "https://www.instapaper.com",
		http:           &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Authenticate() error {
	params := url.Values{
		"x_auth_username": {c.username},
		"x_auth_password": {c.password},
		"x_auth_mode":     {"client_auth"},
	}
	resp, err := c.signedPost("/api/1/oauth/access_token", params)
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authenticate: instapaper returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("authenticate: read response: %w", err)
	}

	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return fmt.Errorf("authenticate: parse response: %w", err)
	}
	c.accessToken = vals.Get("oauth_token")
	c.accessSecret = vals.Get("oauth_token_secret")
	if c.accessToken == "" {
		return fmt.Errorf("authenticate: no oauth_token in response")
	}
	return nil
}

type bookmarkItem struct {
	BookmarkID int64 `json:"bookmark_id"`
}

func (c *Client) Add(itemURL, title string) (int64, error) {
	params := url.Values{"url": {itemURL}}
	if title != "" {
		params.Set("title", title)
	}
	resp, err := c.signedPost("/api/1.1/bookmarks/add", params)
	if err != nil {
		return 0, fmt.Errorf("add bookmark: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("add bookmark: instapaper returned %d for %s", resp.StatusCode, itemURL)
	}

	var items []bookmarkItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return 0, fmt.Errorf("add bookmark: decode response: %w", err)
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("add bookmark: empty response for %s", itemURL)
	}
	return items[0].BookmarkID, nil
}

func (c *Client) Archive(bookmarkID int64) error {
	params := url.Values{"bookmark_id": {strconv.FormatInt(bookmarkID, 10)}}
	resp, err := c.signedPost("/api/1.1/bookmarks/archive", params)
	if err != nil {
		return fmt.Errorf("archive bookmark %d: %w", bookmarkID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("archive bookmark: instapaper returned %d for id %d", resp.StatusCode, bookmarkID)
	}
	return nil
}

func (c *Client) signedPost(endpoint string, params url.Values) (*http.Response, error) {
	nonce := newNonce()
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	sig := sign("POST", c.baseURL+endpoint, params, c.consumerKey, c.consumerSecret, c.accessToken, c.accessSecret, nonce, ts)

	params.Set("oauth_consumer_key", c.consumerKey)
	params.Set("oauth_nonce", nonce)
	params.Set("oauth_signature", sig)
	params.Set("oauth_signature_method", "HMAC-SHA1")
	params.Set("oauth_timestamp", ts)
	params.Set("oauth_version", "1.0")
	if c.accessToken != "" {
		params.Set("oauth_token", c.accessToken)
	}

	return c.http.PostForm(c.baseURL+endpoint, params)
}

func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
