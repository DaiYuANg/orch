package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type apiClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newAPIClient(baseURL, token string, timeout time.Duration) *apiClient {
	return &apiClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func newDefaultAPIClient() (*apiClient, error) {
	token, _, err := resolveToken()
	if err != nil {
		return nil, err
	}
	return newAPIClient(apiAddress, token, requestTimeout), nil
}

func (c *apiClient) Get(path string, out any) error {
	return c.doJSON(http.MethodGet, path, nil, out)
}

func (c *apiClient) Post(path string, in any, out any) error {
	return c.doJSON(http.MethodPost, path, in, out)
}

func (c *apiClient) doJSON(method, path string, in any, out any) error {
	uri := c.baseURL + ensureLeadingSlash(path)

	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s %s failed: status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if out == nil {
		return nil
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && (envelope.Code != 0 || len(envelope.Data) > 0 || envelope.Message != "") {
		if envelope.Code != 0 {
			return fmt.Errorf("api error: code=%d message=%s", envelope.Code, envelope.Message)
		}
		if len(envelope.Data) == 0 {
			return nil
		}
		return json.Unmarshal(envelope.Data, out)
	}

	return json.Unmarshal(raw, out)
}

func buildURLPath(path string, query map[string]string) string {
	base := ensureLeadingSlash(path)
	if len(query) == 0 {
		return base
	}

	values := url.Values{}
	for key, value := range query {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values.Set(key, value)
	}
	if len(values) == 0 {
		return base
	}
	return base + "?" + values.Encode()
}

func resolveToken() (token string, source string, err error) {
	if trimmed := strings.TrimSpace(authToken); trimmed != "" {
		return trimmed, "--token", nil
	}

	if trimmed := strings.TrimSpace(authTokenFile); trimmed != "" {
		content, readErr := os.ReadFile(trimmed)
		if readErr != nil {
			return "", "", readErr
		}
		return strings.TrimSpace(string(content)), trimmed, nil
	}

	defaultPath := filepath.Join(os.TempDir(), "warden.token")
	content, readErr := os.ReadFile(defaultPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", "", nil
		}
		return "", "", readErr
	}
	return strings.TrimSpace(string(content)), defaultPath, nil
}

func ensureLeadingSlash(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}
