package crawlora

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/hex"
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
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultBaseURL = "https://api.crawlora.net/api/v1"
const Version = "1.6.0-sdk.1"

const (
	ResponseAuto   = "auto"
	ResponseJSON   = "json"
	ResponseText   = "text"
	ResponseStream = "stream"
)

type Params map[string]any

type Client struct {
	Services

	APIKey        string
	JWTToken      string
	BaseURL       string
	HTTPClient    *http.Client
	Headers       map[string]string
	Retries       int
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration
	UserAgent     string

	RetryStatuses   map[int]bool
	RetryPredicate  func(status int, err error) bool
	OnRetry         func(attempt int, err error, delay time.Duration)
	RequestID       bool
	IdempotencyKeys bool
	Logger          func(event map[string]any)

	BeforeRequest []func(req *http.Request) error
	AfterResponse []func(operationID string, status int, headers http.Header, body any) (any, error)

	rateLimiter *rateLimiter
	concurrency chan struct{}
}

// rateLimiter spaces requests by a fixed minimum interval (requests per second).
type rateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newRateLimiter(perSecond float64) *rateLimiter {
	return &rateLimiter{interval: time.Duration(float64(time.Second) / perSecond)}
}

func (r *rateLimiter) wait(ctx context.Context) error {
	r.mu.Lock()
	now := time.Now()
	if r.next.Before(now) {
		r.next = now
	}
	delay := r.next.Sub(now)
	r.next = r.next.Add(r.interval)
	r.mu.Unlock()
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

type Option func(*Client)
type RequestOption func(*requestConfig)

type requestConfig struct {
	Headers        map[string]string
	ResponseType   string
	Timeout        time.Duration
	Retries        *int
	RetryPredicate func(status int, err error) bool
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

func String(value string) *string {
	return &value
}

func Int(value int) *int {
	return &value
}

func Bool(value bool) *bool {
	return &value
}

func Float64(value float64) *float64 {
	return &value
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
	Status     int
	Code       int
	Message    string
	Body       any
	RawBody    string
	Headers    http.Header
	RetryAfter time.Duration
	RequestID  string
	Err        error
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

// Sentinel errors for classifying failures with errors.Is. A *Error reports
// itself as one of these based on its status:
//
//	if errors.Is(err, crawlora.ErrServer) { /* retry or alert */ }
var (
	ErrClient  = errors.New("crawlora: client error")  // 4xx response
	ErrServer  = errors.New("crawlora: server error")  // 5xx response
	ErrNetwork = errors.New("crawlora: network error") // transport failure before a response
)

// IsClientError reports whether the API rejected the request (4xx).
func (e *Error) IsClientError() bool { return e.Status >= 400 && e.Status < 500 }

// IsServerError reports whether the API failed to handle a valid request (5xx).
func (e *Error) IsServerError() bool { return e.Status >= 500 }

// IsNetworkError reports whether the request failed before a response arrived.
func (e *Error) IsNetworkError() bool { return e.Status == 0 && e.Err != nil }

func (e *Error) Is(target error) bool {
	switch target {
	case ErrClient:
		return e.IsClientError()
	case ErrServer:
		return e.IsServerError()
	case ErrNetwork:
		return e.IsNetworkError()
	}
	return false
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		BaseURL:       DefaultBaseURL,
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
		Headers:       map[string]string{},
		RetryDelay:    250 * time.Millisecond,
		MaxRetryDelay: 30 * time.Second,
		UserAgent:     "crawlora-go-sdk/" + Version,
	}
	for _, opt := range opts {
		opt(c)
	}
	// Precedence: explicit option > environment variable > default.
	if c.APIKey == "" {
		c.APIKey = os.Getenv("CRAWLORA_API_KEY")
	}
	if c.BaseURL == DefaultBaseURL {
		if env := os.Getenv("CRAWLORA_BASE_URL"); env != "" {
			c.BaseURL = env
		}
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.Retries < 0 {
		c.Retries = 0
	}
	if c.RetryDelay < 0 {
		c.RetryDelay = 0
	}
	if c.MaxRetryDelay < 0 {
		c.MaxRetryDelay = 0
	}
	c.Services = initServices(c)
	return c
}

// WithMaxRetryDelay caps backoff and Retry-After delays (default 30s).
func WithMaxRetryDelay(delay time.Duration) Option {
	return func(c *Client) { c.MaxRetryDelay = delay }
}

// WithRetryableStatuses overrides the retryable HTTP status set. Network
// failures (status 0) stay retryable unless a predicate decides otherwise.
func WithRetryableStatuses(statuses ...int) Option {
	return func(c *Client) {
		c.RetryStatuses = map[int]bool{}
		for _, s := range statuses {
			c.RetryStatuses[s] = true
		}
	}
}

// WithRetryPredicate sets a full retry predicate; it supersedes the status set.
func WithRetryPredicate(predicate func(status int, err error) bool) Option {
	return func(c *Client) { c.RetryPredicate = predicate }
}

// WithOnRetry registers a hook invoked before each retry sleep.
func WithOnRetry(hook func(attempt int, err error, delay time.Duration)) Option {
	return func(c *Client) { c.OnRetry = hook }
}

// WithRequestID enables generating an x-request-id header when absent.
func WithRequestID(enabled bool) Option {
	return func(c *Client) { c.RequestID = enabled }
}

// WithIdempotencyKeys attaches a stable Idempotency-Key header to POST/PATCH
// requests, reused across that call's retries.
func WithIdempotencyKeys(enabled bool) Option {
	return func(c *Client) { c.IdempotencyKeys = enabled }
}

// WithRateLimit caps outgoing requests to at most perSecond requests per second.
func WithRateLimit(perSecond float64) Option {
	return func(c *Client) {
		if perSecond > 0 {
			c.rateLimiter = newRateLimiter(perSecond)
		}
	}
}

// WithMaxConcurrency caps the number of in-flight requests.
func WithMaxConcurrency(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.concurrency = make(chan struct{}, n)
		}
	}
}

// WithRequestRetries overrides the client retry count for a single request.
func WithRequestRetries(retries int) RequestOption {
	return func(cfg *requestConfig) { cfg.Retries = &retries }
}

// WithRequestRetryPredicate overrides the retry predicate for a single request.
func WithRequestRetryPredicate(predicate func(status int, err error) bool) RequestOption {
	return func(cfg *requestConfig) { cfg.RetryPredicate = predicate }
}

// WithLogger registers a structured event sink (request/retry). The SDK never
// logs on its own.
func WithLogger(logger func(event map[string]any)) Option {
	return func(c *Client) { c.Logger = logger }
}

// WithBeforeRequest appends a hook that runs just before each request is sent
// and may mutate the *http.Request (headers, URL, body). A returned error aborts
// the request.
func WithBeforeRequest(hook func(req *http.Request) error) Option {
	return func(c *Client) { c.BeforeRequest = append(c.BeforeRequest, hook) }
}

// WithAfterResponse appends a hook that runs on a successful parsed response and
// may return a replacement body. A returned error aborts the request.
func WithAfterResponse(hook func(operationID string, status int, headers http.Header, body any) (any, error)) Option {
	return func(c *Client) { c.AfterResponse = append(c.AfterResponse, hook) }
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

func RequestTyped[T any](c *Client, ctx context.Context, operationID string, params Params, opts ...RequestOption) (T, error) {
	return requestTyped[T](c, ctx, operationID, params, opts...)
}

func requestTyped[T any](c *Client, ctx context.Context, operationID string, params Params, opts ...RequestOption) (T, error) {
	var zero T
	out, err := c.Request(ctx, operationID, params, opts...)
	if err != nil {
		return zero, err
	}
	body, err := json.Marshal(out)
	if err != nil {
		return zero, &Error{Message: "crawlora typed response encode error", Err: err}
	}
	var typed T
	if err := json.Unmarshal(body, &typed); err != nil {
		return zero, &Error{Message: "crawlora typed response decode error", Err: err}
	}
	return typed, nil
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
	if err := validateResponseType(cfg.ResponseType); err != nil {
		return nil, err
	}
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	c.log(map[string]any{"event": "request", "operation": operationID})
	maxRetries := c.Retries
	if cfg.Retries != nil {
		maxRetries = *cfg.Retries
		if maxRetries < 0 {
			maxRetries = 0
		}
	}
	idempotencyKey := ""
	if c.IdempotencyKeys && (operation.Method == "POST" || operation.Method == "PATCH") {
		idempotencyKey = newRequestID()
	}
	var last error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		out, err := c.send(ctx, operationID, operation, params, cfg, idempotencyKey)
		if err == nil {
			return out, nil
		}
		last = err
		if !c.isRetryableWith(err, cfg.RetryPredicate) || attempt == maxRetries {
			break
		}
		delay := c.retryDelayForError(err, attempt)
		c.log(map[string]any{"event": "retry", "operation": operationID, "attempt": attempt + 1, "delay": delay})
		if c.OnRetry != nil {
			c.OnRetry(attempt+1, err, delay)
		}
		if err := sleepBeforeRetry(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func (c *Client) log(event map[string]any) {
	if c.Logger != nil {
		c.Logger(event)
	}
}

func (c *Client) send(ctx context.Context, operationID string, operation operationDefinition, params Params, cfg requestConfig, idempotencyKey string) (any, error) {
	if params == nil {
		params = Params{}
	}
	if c.concurrency != nil {
		select {
		case c.concurrency <- struct{}{}:
			defer func() { <-c.concurrency }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if c.rateLimiter != nil {
		if err := c.rateLimiter.wait(ctx); err != nil {
			return nil, err
		}
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
				if !hasAuthScheme(token) {
					token = "Token " + token
				}
				req.Header.Set("Authorization", token)
			}
		}
	}
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}
	reqID := req.Header.Get("x-request-id")
	if c.RequestID && reqID == "" {
		reqID = newRequestID()
		req.Header.Set("x-request-id", reqID)
	}
	if idempotencyKey != "" && req.Header.Get("Idempotency-Key") == "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	for _, hook := range c.BeforeRequest {
		if err := hook(req); err != nil {
			return nil, err
		}
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, &Error{Message: "crawlora transport error", RequestID: reqID, Err: err}
	}

	// Streaming success returns the unread body; the caller must close it.
	if cfg.ResponseType == ResponseStream && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.Body, nil
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Message: "crawlora response read error", Headers: resp.Header.Clone(), RequestID: reqID, Err: err}
	}
	parseMode := cfg.ResponseType
	if parseMode == ResponseStream {
		parseMode = ResponseAuto // stream only reaches here on an error status; parse the error body
	}
	parsed, parseErr := parseResponse(responseBody, resp.Header.Get("content-type"), parseMode)
	if parseErr != nil {
		return nil, &Error{
			Status:    resp.StatusCode,
			Message:   "crawlora JSON parse error",
			RawBody:   string(responseBody),
			Headers:   resp.Header.Clone(),
			RequestID: reqID,
			Err:       parseErr,
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &Error{
			Status:     resp.StatusCode,
			Body:       parsed,
			RawBody:    string(responseBody),
			Headers:    resp.Header.Clone(),
			RetryAfter: retryAfterDelay(resp.Header, c.MaxRetryDelay),
			RequestID:  reqID,
		}
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
	for _, hook := range c.AfterResponse {
		replacement, err := hook(operationID, resp.StatusCode, resp.Header, parsed)
		if err != nil {
			return nil, err
		}
		if replacement != nil {
			parsed = replacement
		}
	}
	return parsed, nil
}

func buildRequest(baseURL string, operation operationDefinition, params Params) (string, io.Reader, string, error) {
	if err := validateRequiredParams(operation, params); err != nil {
		return "", nil, "", err
	}
	if err := validateEnumParams(operation, params); err != nil {
		return "", nil, "", err
	}
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

func validateRequiredParams(operation operationDefinition, params Params) error {
	for _, name := range operation.PathParams {
		if missingParam(params[name]) {
			return fmt.Errorf("missing required path parameter: %s", name)
		}
	}
	for _, parameter := range operation.QueryParams {
		if parameter.Required && missingParam(params[parameter.Name]) {
			return fmt.Errorf("missing required %s parameter: %s", parameter.In, parameter.Name)
		}
	}
	for _, parameter := range operation.FormParams {
		if parameter.Required && missingParam(params[parameter.Name]) {
			return fmt.Errorf("missing required %s parameter: %s", parameter.In, parameter.Name)
		}
	}
	if operation.BodyRequired && missingParam(params[operation.BodyParam]) && missingParam(params["body"]) {
		return fmt.Errorf("missing required body parameter: %s", operation.BodyParam)
	}
	return nil
}

func validateEnumParams(operation operationDefinition, params Params) error {
	for _, parameter := range operation.QueryParams {
		if err := validateEnumParam(parameter, params[parameter.Name]); err != nil {
			return err
		}
	}
	for _, parameter := range operation.FormParams {
		if err := validateEnumParam(parameter, params[parameter.Name]); err != nil {
			return err
		}
	}
	return nil
}

func validateEnumParam(parameter parameterDefinition, value any) error {
	if len(parameter.Enum) == 0 || missingParam(value) {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, enumValue := range parameter.Enum {
		allowed[enumValue] = struct{}{}
	}
	for _, item := range queryValues(value) {
		if _, ok := allowed[item]; !ok {
			return fmt.Errorf("invalid %s parameter %s: expected one of %s", parameter.In, parameter.Name, strings.Join(parameter.Enum, ", "))
		}
	}
	return nil
}

func missingParam(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String:
		return rv.Len() == 0
	case reflect.Slice, reflect.Array:
		return rv.Len() == 0
	}
	return false
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

func parseResponse(body []byte, contentType string, responseType string) (any, error) {
	if responseType == ResponseText {
		return string(body), nil
	}
	if responseType == ResponseJSON || strings.Contains(strings.ToLower(contentType), "application/json") {
		var out any
		if len(body) == 0 {
			return nil, nil
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	return string(body), nil
}

func validateResponseType(responseType string) error {
	switch responseType {
	case ResponseAuto, ResponseJSON, ResponseText, ResponseStream:
		return nil
	default:
		return fmt.Errorf("invalid response type: expected one of auto, json, text, stream")
	}
}

func shouldRetry(status int) bool {
	return status == 408 || status == 409 || status == 425 || status == 429 || status >= 500
}

func hasAuthScheme(token string) bool {
	return len(token) >= 6 && strings.EqualFold(token[:6], "Token ") ||
		len(token) >= 7 && strings.EqualFold(token[:7], "Bearer ")
}

func newRequestID() string {
	var buf [16]byte
	if _, err := crand.Read(buf[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

func (c *Client) isRetryable(err error) bool {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		return false
	}
	if c.RetryPredicate != nil {
		return c.RetryPredicate(apiErr.Status, err)
	}
	if c.RetryStatuses != nil {
		// Network failures (status 0) stay retryable unless a predicate decides.
		return apiErr.Status == 0 || c.RetryStatuses[apiErr.Status]
	}
	if apiErr.Status == 0 {
		return true
	}
	return shouldRetry(apiErr.Status)
}

// isRetryableWith applies a per-request predicate when provided, else the
// client's default retry policy.
func (c *Client) isRetryableWith(err error, predicate func(status int, err error) bool) bool {
	if predicate != nil {
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			return false
		}
		return predicate(apiErr.Status, err)
	}
	return c.isRetryable(err)
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

func (c *Client) retryDelayForError(err error, attempt int) time.Duration {
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter
	}
	return c.retryDelay(attempt)
}

func retryAfterDelay(headers http.Header, maxDelay time.Duration) time.Duration {
	value := headers.Get("Retry-After")
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds > 0 {
		return minDuration(time.Duration(seconds*float64(time.Second)), maxDelay)
	}
	if target, err := http.ParseTime(value); err == nil {
		delay := time.Until(target)
		if delay > 0 {
			return minDuration(delay, maxDelay)
		}
	}
	return 0
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
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

// ErrStopPagination, returned from a Paginate callback, stops iteration without
// reporting an error.
var ErrStopPagination = errors.New("crawlora: stop pagination")

type paginateConfig struct {
	pageParam   string
	start       int
	hasStart    bool
	step        int
	maxPages    int
	cursorParam string
	cursorStart any
	nextCursor  func(page any) any
	items       func(page any) []any
}

type PaginateOption func(*paginateConfig)

// WithPageParam overrides the auto-detected page/offset query parameter.
func WithPageParam(name string) PaginateOption {
	return func(cfg *paginateConfig) { cfg.pageParam = name }
}

// WithCursorParam enables cursor pagination using the named query parameter.
// Requires WithNextCursor.
func WithCursorParam(name string) PaginateOption {
	return func(cfg *paginateConfig) { cfg.cursorParam = name }
}

// WithNextCursor sets the extractor that returns the next cursor from a page;
// iteration stops when it returns nil.
func WithNextCursor(fn func(page any) any) PaginateOption {
	return func(cfg *paginateConfig) { cfg.nextCursor = fn }
}

// WithCursorStart sets the initial cursor value (cursor mode).
func WithCursorStart(value any) PaginateOption {
	return func(cfg *paginateConfig) { cfg.cursorStart = value }
}

// WithItems sets the per-page item extractor for PaginateItems (default: the
// "data" array).
func WithItems(fn func(page any) []any) PaginateOption {
	return func(cfg *paginateConfig) { cfg.items = fn }
}

// WithPageStart sets the first page value (defaults to 1 for page, 0 for offset).
func WithPageStart(start int) PaginateOption {
	return func(cfg *paginateConfig) {
		cfg.start = start
		cfg.hasStart = true
	}
}

// WithPageStep sets the amount added to the page value after each page (default 1).
func WithPageStep(step int) PaginateOption {
	return func(cfg *paginateConfig) { cfg.step = step }
}

// WithMaxPages caps the number of pages fetched.
func WithMaxPages(maxPages int) PaginateOption {
	return func(cfg *paginateConfig) { cfg.maxPages = maxPages }
}

// Paginate walks pages of a paginated operation, invoking fn for each page. It
// advances the numeric page/offset query parameter and stops when a page
// returns no data, when fn returns ErrStopPagination, or when fn returns any
// other error (which is propagated).
func (c *Client) Paginate(ctx context.Context, operationID string, params Params, fn func(page any) error, opts ...PaginateOption) error {
	operation, ok := operations[operationID]
	if !ok {
		return fmt.Errorf("unknown Crawlora operation: %s", operationID)
	}
	cfg := paginateConfig{step: 1, maxPages: -1}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.cursorParam != "" || cfg.nextCursor != nil {
		if cfg.cursorParam == "" || cfg.nextCursor == nil {
			return fmt.Errorf("cursor pagination requires both WithCursorParam and WithNextCursor")
		}
		found := false
		for _, parameter := range operation.QueryParams {
			if parameter.Name == cfg.cursorParam {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("cursor parameter %q is not a query parameter of operation %s", cfg.cursorParam, operationID)
		}
		cursor := cfg.cursorStart
		for fetched := 0; cfg.maxPages < 0 || fetched < cfg.maxPages; fetched++ {
			pageParams := Params{}
			for key, value := range params {
				pageParams[key] = value
			}
			if cursor != nil {
				pageParams[cfg.cursorParam] = cursor
			}
			page, err := c.Request(ctx, operationID, pageParams)
			if err != nil {
				return err
			}
			if err := fn(page); err != nil {
				if errors.Is(err, ErrStopPagination) {
					return nil
				}
				return err
			}
			cursor = cfg.nextCursor(page)
			if cursor == nil {
				break
			}
		}
		return nil
	}

	pageParam := cfg.pageParam
	if pageParam == "" {
		pageParam = detectPageParam(operation)
	}
	if pageParam == "" {
		return fmt.Errorf("operation %s has no page or offset query parameter to paginate", operationID)
	}
	start := cfg.start
	if !cfg.hasStart && pageParam == "offset" {
		start = 0
	} else if !cfg.hasStart {
		start = 1
	}
	step := cfg.step
	if step == 0 {
		step = 1
	}
	pageValue := start
	for fetched := 0; cfg.maxPages < 0 || fetched < cfg.maxPages; fetched++ {
		pageParams := Params{}
		for key, value := range params {
			pageParams[key] = value
		}
		pageParams[pageParam] = pageValue
		page, err := c.Request(ctx, operationID, pageParams)
		if err != nil {
			return err
		}
		if err := fn(page); err != nil {
			if errors.Is(err, ErrStopPagination) {
				return nil
			}
			return err
		}
		if pageIsEmpty(page) {
			break
		}
		pageValue += step
	}
	return nil
}

// PaginateItems walks pages and invokes fn for each item. Items are extracted
// per page (default: the "data" array; override with WithItems). Return
// ErrStopPagination from fn to stop early.
func (c *Client) PaginateItems(ctx context.Context, operationID string, params Params, fn func(item any) error, opts ...PaginateOption) error {
	cfg := paginateConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	extract := cfg.items
	if extract == nil {
		extract = defaultItems
	}
	return c.Paginate(ctx, operationID, params, func(page any) error {
		for _, item := range extract(page) {
			if err := fn(item); err != nil {
				return err
			}
		}
		return nil
	}, opts...)
}

func defaultItems(page any) []any {
	data := page
	if body, ok := page.(map[string]any); ok {
		if value, exists := body["data"]; exists {
			data = value
		}
	}
	if list, ok := data.([]any); ok {
		return list
	}
	return nil
}

func detectPageParam(operation operationDefinition) string {
	for _, name := range []string{"page", "offset"} {
		for _, parameter := range operation.QueryParams {
			if parameter.Name == name {
				return name
			}
		}
	}
	return ""
}

func pageIsEmpty(page any) bool {
	data := page
	if body, ok := page.(map[string]any); ok {
		if value, exists := body["data"]; exists {
			data = value
		}
	}
	switch value := data.(type) {
	case nil:
		return true
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	case string:
		return value == ""
	default:
		return false
	}
}
