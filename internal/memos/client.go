package memos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the Memos REST API.
type Client struct {
	BaseURL    string
	APIToken   string
	HTTPClient *http.Client
}

// NewClient creates a Memos API client.
func NewClient(baseURL, apiToken string) *Client {
	return &Client{
		BaseURL:  baseURL,
		APIToken: apiToken,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Memo represents a Memos memo object.
type Memo struct {
	Name       string `json:"name,omitempty"`
	UID        string `json:"uid,omitempty"`
	Content    string `json:"content"`
	Visibility string `json:"visibility,omitempty"` // "PRIVATE", "PROTECTED", "PUBLIC"
	Pinned     bool   `json:"pinned,omitempty"`
}

// MemoResponse is the response from creating/listing memos.
type MemoResponse struct {
	Name       string `json:"name"`
	UID        string `json:"uid"`
	Content    string `json:"content"`
	Visibility string `json:"visibility"`
	Pinned     bool   `json:"pinned"`
}

// ListMemosResponse is the response from listing memos.
type ListMemosResponse struct {
	Memos         []MemoResponse `json:"memos"`
	NextPageToken string         `json:"nextPageToken"`
}

// CreateMemo creates a new memo.
func (c *Client) CreateMemo(memo Memo) (*MemoResponse, error) {
	body, _ := json.Marshal(memo)
	req, err := http.NewRequest("POST", c.BaseURL+"/api/v1/memos", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("memos create: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("memos create: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result MemoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMemos lists memos with an optional filter.
func (c *Client) ListMemos(filter string, pageSize int) ([]MemoResponse, error) {
	url := fmt.Sprintf("%s/api/v1/memos?pageSize=%d", c.BaseURL, pageSize)
	if filter != "" {
		url += "&filter=" + filter
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("memos list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("memos list: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result ListMemosResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Memos, nil
}

// DeleteMemo deletes a memo by its resource name.
func (c *Client) DeleteMemo(name string) error {
	req, err := http.NewRequest("DELETE", c.BaseURL+"/api/v1/"+name, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("memos delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("memos delete: status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// Ping checks if the Memos server is reachable.
func (c *Client) Ping() error {
	req, err := http.NewRequest("GET", c.BaseURL+"/api/v1/workspace/profile", nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("memos ping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("memos ping: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	}
}
