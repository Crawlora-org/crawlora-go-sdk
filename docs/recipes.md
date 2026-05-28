# Crawlora Go SDK Recipes

## Authentication

```go
client := crawlora.NewClient(crawlora.WithAPIKey(os.Getenv("CRAWLORA_API_KEY")))
```

Self-service account endpoints can use JWT auth:

```go
client := crawlora.NewClient(crawlora.WithJWTToken(os.Getenv("CRAWLORA_JWT_TOKEN")))
```

## Typed Endpoints

```go
result, err := client.Bing.SearchTyped(ctx, crawlora.BingSearchParams{
    Q:     "coffee shops",
    Count: crawlora.Int(10),
})
```

Use `crawlora.String`, `crawlora.Int`, `crawlora.Bool`, and
`crawlora.Float64` for optional typed parameters.

## Typed Dynamic Operations

```go
result, err := crawlora.RequestTyped[crawlora.BingSearchResponse](
    client,
    ctx,
    crawlora.OperationBingSearch,
    crawlora.Params{"q": "coffee shops"},
)
```

## Retries, Timeouts, And Headers

```go
client := crawlora.NewClient(
    crawlora.WithAPIKey(os.Getenv("CRAWLORA_API_KEY")),
    crawlora.WithRetries(2),
    crawlora.WithRetryDelay(250*time.Millisecond),
)

result, err := client.Bing.Search(
    ctx,
    crawlora.Params{"q": "coffee shops"},
    crawlora.WithRequestTimeout(10*time.Second),
    crawlora.WithRequestHeader("x-client", "example"),
)
```

Request headers override default auth, user-agent, and content headers
case-insensitively. Retryable API responses honor positive `Retry-After`
headers, capped at 30 seconds.

## Text Responses

Response mode must be `crawlora.ResponseAuto`, `crawlora.ResponseJSON`, or
`crawlora.ResponseText`.

```go
text, err := client.YouTube.Transcript(
    ctx,
    crawlora.Params{"id": "VIDEO_ID", "format": "text"},
    crawlora.WithResponseType(crawlora.ResponseText),
)
```

## Errors

```go
var apiErr *crawlora.Error
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.Status, apiErr.Code, apiErr.RawBody, apiErr.Headers)
}
```

Context cancellation and deadline errors are returned directly for
`errors.Is` checks.

## Optional Live Smoke Tests

```sh
CRAWLORA_API_KEY=... go run ./examples/bing-search
CRAWLORA_API_KEY=... CRAWLORA_YOUTUBE_VIDEO_ID=... go run ./examples/youtube-transcript
```
