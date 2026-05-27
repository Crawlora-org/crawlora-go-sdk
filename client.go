package crawlora

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.crawlora.net/api/v1"

type Params map[string]any

type Client struct {
	Services

	APIKey     string
	JWTToken   string
	BaseURL    string
	HTTPClient *http.Client
	Headers    map[string]string
	Retries    int
}

type Option func(*Client)

type Error struct {
	Status int
	Code   int
	Body   any
}

func (e *Error) Error() string {
	if body, ok := e.Body.(map[string]any); ok {
		if msg, ok := body["msg"].(string); ok && msg != "" {
			return msg
		}
	}
	return fmt.Sprintf("crawlora request failed with status %d", e.Status)
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Headers:    map[string]string{},
	}
	for _, opt := range opts {
		opt(c)
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	c.Services = initServices(c)
	return c
}

func WithAPIKey(apiKey string) Option {
	return func(c *Client) { c.APIKey = apiKey }
}

func WithJWTToken(token string) Option {
	return func(c *Client) { c.JWTToken = token }
}

func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.BaseURL = baseURL }
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.HTTPClient = client
		}
	}
}

func WithHeader(name, value string) Option {
	return func(c *Client) { c.Headers[name] = value }
}

func WithRetries(retries int) Option {
	return func(c *Client) { c.Retries = retries }
}

func (c *Client) Operation(ctx context.Context, operationID string, params Params) (any, error) {
	return c.Request(ctx, operationID, params)
}

func (c *Client) Request(ctx context.Context, operationID string, params Params) (any, error) {
	operation, ok := operations[operationID]
	if !ok {
		return nil, fmt.Errorf("unknown Crawlora operation: %s", operationID)
	}
	var last error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		out, err := c.send(ctx, operation, params)
		if err == nil {
			return out, nil
		}
		last = err
		if apiErr, ok := err.(*Error); !ok || !shouldRetry(apiErr.Status) {
			break
		}
	}
	return nil, last
}

func (c *Client) send(ctx context.Context, operation operationDefinition, params Params) (any, error) {
	if params == nil {
		params = Params{}
	}
	requestURL, body, contentType, err := buildRequest(c.BaseURL, operation, params)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, operation.Method, requestURL, body)
	if err != nil {
		return nil, err
	}
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}
	for _, security := range operation.Security {
		switch security {
		case "ApiKeyAuth":
			if c.APIKey != "" {
				req.Header.Set("x-api-key", c.APIKey)
			}
		case "JWTAuth":
			if c.JWTToken != "" {
				token := c.JWTToken
				if !strings.HasPrefix(token, "Token ") && !strings.HasPrefix(token, "Bearer ") {
					token = "Token " + token
				}
				req.Header.Set("Authorization", token)
			}
		}
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	parsed := parseResponse(responseBody, resp.Header.Get("content-type"))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &Error{Status: resp.StatusCode, Body: parsed}
		if body, ok := parsed.(map[string]any); ok {
			if code, ok := body["code"].(float64); ok {
				apiErr.Code = int(code)
			}
		}
		return nil, apiErr
	}
	return parsed, nil
}

func buildRequest(baseURL string, operation operationDefinition, params Params) (string, io.Reader, string, error) {
	path := operation.Path
	for _, name := range operation.PathParams {
		value, ok := params[name]
		if !ok || value == nil || fmt.Sprint(value) == "" {
			return "", nil, "", fmt.Errorf("missing required path parameter: %s", name)
		}
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(fmt.Sprint(value)))
	}

	query := url.Values{}
	for _, parameter := range operation.QueryParams {
		value := params[parameter.Name]
		if value == nil || fmt.Sprint(value) == "" {
			continue
		}
		if values, ok := value.([]string); ok {
			for _, item := range values {
				query.Add(parameter.Name, item)
			}
			continue
		}
		query.Add(parameter.Name, fmt.Sprint(value))
	}
	requestURL := baseURL + path
	if encoded := query.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	if len(operation.FormParams) > 0 {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		for _, parameter := range operation.FormParams {
			value := params[parameter.Name]
			if value == nil {
				continue
			}
			if parameter.Type == "file" {
				if err := writeFilePart(writer, parameter.Name, value); err != nil {
					return "", nil, "", err
				}
				continue
			}
			if err := writer.WriteField(parameter.Name, fmt.Sprint(value)); err != nil {
				return "", nil, "", err
			}
		}
		if err := writer.Close(); err != nil {
			return "", nil, "", err
		}
		return requestURL, body, writer.FormDataContentType(), nil
	}

	if operation.BodyParam != "" {
		value := params[operation.BodyParam]
		if value == nil {
			value = params["body"]
		}
		if value != nil {
			body, err := json.Marshal(value)
			if err != nil {
				return "", nil, "", err
			}
			return requestURL, bytes.NewReader(body), "application/json", nil
		}
	}

	return requestURL, nil, "", nil
}

func writeFilePart(writer *multipart.Writer, name string, value any) error {
	switch v := value.(type) {
	case []byte:
		part, err := writer.CreateFormFile(name, "upload.bin")
		if err != nil {
			return err
		}
		_, err = part.Write(v)
		return err
	case string:
		file, err := os.Open(v)
		if err != nil {
			return err
		}
		defer file.Close()
		part, err := writer.CreateFormFile(name, filepath.Base(v))
		if err != nil {
			return err
		}
		_, err = io.Copy(part, file)
		return err
	case io.Reader:
		part, err := writer.CreateFormFile(name, "upload.bin")
		if err != nil {
			return err
		}
		_, err = io.Copy(part, v)
		return err
	default:
		return fmt.Errorf("unsupported file value for %s", name)
	}
}

func parseResponse(body []byte, contentType string) any {
	if strings.Contains(contentType, "application/json") {
		var out any
		if err := json.Unmarshal(body, &out); err == nil {
			return out
		}
	}
	return string(body)
}

func shouldRetry(status int) bool {
	return status == 408 || status == 409 || status == 425 || status == 429 || status >= 500
}
