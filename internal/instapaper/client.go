package instapaper

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	username string
	password string
	baseURL  string
	http     *http.Client
}

func NewClient(username, password string) *Client {
	return &Client{
		username: username,
		password: password,
		baseURL:  "https://www.instapaper.com",
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Add(itemURL, title string) error {
	params := url.Values{
		"username": {c.username},
		"password": {c.password},
		"url":      {itemURL},
	}
	if title != "" {
		params.Set("title", title)
	}

	resp, err := c.http.PostForm(c.baseURL+"/api/add", params)
	if err != nil {
		return fmt.Errorf("post to instapaper: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("instapaper returned %d for %s", resp.StatusCode, itemURL)
	}
	return nil
}
