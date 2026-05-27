package crawlora

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.crawlora.net/api/v1"
const Version = "1.2.0-sdk.5"

const (
	ResponseAuto = "auto"
	ResponseJSON = "json"
	ResponseText = "text"
)

type Params map[string]any

type Client struct {
	Services

	APIKey     string
	JWTToken   string
	BaseURL    string
	HTTPClient *http.Client
	Headers    map[string]string
	Retries    int
	RetryDelay time.Duration
	UserAgent  string
}

type Option func(*Client)
type RequestOption func(*requestConfig)

type requestConfig struct {
	Headers      map[string]string
	ResponseType string
	Timeout      time.Duration
}

func paramsFromStruct(input any) Params {
	if input == nil {
		return nil
	}
	if params, ok := input.(Params); ok {
		return params
	}
	value := reflect.ValueOf(input)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return Params{"body": input}
	}
	typ := value.Type()
	params := Params{}
	for i := 0; i < value.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("crawlora")
		if tag == "" || tag == "-" {
			continue
		}
		tagParts := strings.Split(tag, ",")
		name := tagParts[0]
		if name == "" {
			continue
		}
		fieldValue := value.Field(i)
		if tagHasOption(tagParts, "omitempty") && isEmptyValue(fieldValue) {
			continue
		}
		if fieldValue.Kind() == reflect.Pointer {
			if fieldValue.IsNil() {
				continue
			}
			fieldValue = fieldValue.Elem()
		}
		if fieldValue.CanInterface() {
			params[name] = fieldValue.Interface()
		}
	}
	return params
}

func tagHasOption(parts []string, option string) bool {
	for _, part := range parts[1:] {
		if part == option {
			return true
		}
	}
	return false
}

func isEmptyValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return value.IsNil()
	}
	return false
}

type Error struct {
	Status  int
	Code    int
	Message string
	Body    any
	RawBody string
	Err     error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("crawlora request failed with status %d", e.Status)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Headers:    map[string]string{},
		RetryDelay: 250 * time.Millisecond,
		UserAgent:  "crawlora-go-sdk/" + Version,
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

func WithRetryDelay(delay time.Duration) Option {
	return func(c *Client) { c.RetryDelay = delay }
}

func WithUserAgent(userAgent string) Option {
	return func(c *Client) { c.UserAgent = userAgent }
}

func WithRequestHeader(name, value string) RequestOption {
	return func(cfg *requestConfig) {
		if cfg.Headers == nil {
			cfg.Headers = map[string]string{}
		}
		cfg.Headers[name] = value
	}
}

func WithResponseType(responseType string) RequestOption {
	return func(cfg *requestConfig) { cfg.ResponseType = responseType }
}

func WithRequestTimeout(timeout time.Duration) RequestOption {
	return func(cfg *requestConfig) { cfg.Timeout = timeout }
}

func (c *Client) Operation(ctx context.Context, operationID string, params Params, opts ...RequestOption) (any, error) {
	return c.Request(ctx, operationID, params, opts...)
}

func (c *Client) Request(ctx context.Context, operationID string, params Params, opts ...RequestOption) (any, error) {
	operation, ok := operations[operationID]
	if !ok {
		return nil, fmt.Errorf("unknown Crawlora operation: %s", operationID)
	}
	cfg := requestConfig{ResponseType: ResponseAuto}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	var last error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		out, err := c.send(ctx, operation, params, cfg)
		if err == nil {
			return out, nil
		}
		last = err
		if !isRetryableError(err) || attempt == c.Retries {
			break
		}
		if err := sleepBeforeRetry(ctx, c.retryDelay(attempt)); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func (c *Client) send(ctx context.Context, operation operationDefinition, params Params, cfg requestConfig) (any, error) {
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
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}
	if c.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.UserAgent)
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
		return nil, &Error{Message: "crawlora transport error", Err: err}
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Message: "crawlora response read error", Err: err}
	}
	parsed := parseResponse(responseBody, resp.Header.Get("content-type"), cfg.ResponseType)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &Error{Status: resp.StatusCode, Body: parsed, RawBody: string(responseBody)}
		if body, ok := parsed.(map[string]any); ok {
			if code, ok := body["code"].(float64); ok {
				apiErr.Code = int(code)
			}
			if msg, ok := body["msg"].(string); ok {
				apiErr.Message = msg
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
		if value == nil || (reflect.TypeOf(value).Kind() == reflect.String && fmt.Sprint(value) == "") {
			continue
		}
		for _, item := range queryValues(value) {
			query.Add(parameter.Name, item)
		}
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

func queryValues(value any) []string {
	rv := reflect.ValueOf(value)
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		values := make([]string, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			item := rv.Index(i).Interface()
			if item != nil {
				values = append(values, fmt.Sprint(item))
			}
		}
		return values
	}
	return []string{fmt.Sprint(value)}
}

func parseResponse(body []byte, contentType string, responseType string) any {
	if responseType == ResponseText {
		return string(body)
	}
	if responseType == ResponseJSON || strings.Contains(strings.ToLower(contentType), "application/json") {
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

func isRetryableError(err error) bool {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.Status == 0 {
		return true
	}
	return shouldRetry(apiErr.Status)
}

func (c *Client) retryDelay(attempt int) time.Duration {
	if c.RetryDelay <= 0 {
		return 0
	}
	delay := c.RetryDelay << attempt
	jitterMax := int64(c.RetryDelay / 2)
	if jitterMax <= 0 {
		return delay
	}
	jitter := time.Duration(rand.Int63n(jitterMax))
	return delay + jitter
}

func sleepBeforeRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
