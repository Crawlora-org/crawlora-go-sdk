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

## Reddit And Brand

Newer platforms are grouped like every other endpoint:

```go
posts, err := client.Reddit.Search(ctx, crawlora.Params{"q": "golang", "subreddit": "programming"})
brand, err := client.Brand.Retrieve(ctx, crawlora.Params{"domain": "stripe.com"})
```



## Airbnb Markets Dataset

Aggregate Airbnb short-term-rental market data — listing supply, ratings and nightly-price bands rolled up by country, metro and geo cell. Aggregate-only.

```go
markets, err := client.Datasets.AirbnbMarketsSearch(ctx, crawlora.Params{"group_by": "country", "sort": "listings_desc"})
fr, err := client.Datasets.AirbnbMarketsItem(ctx, crawlora.Params{"country": "FR"})
density, err := client.Datasets.AirbnbMarketsNearby(ctx, crawlora.Params{"lat": 48.86, "lon": 2.35, "radius_m": 5000})
```

## Airbnb Markets Dataset

Aggregate Airbnb short-term-rental market data — listing supply, ratings and nightly-price bands rolled up by country, metro and geo cell. Aggregate-only.

```go
markets, err := client.Datasets.AirbnbMarketsSearch(ctx, crawlora.Params{"group_by": "country", "sort": "listings_desc"})
fr, err := client.Datasets.AirbnbMarketsItem(ctx, crawlora.Params{"country": "FR"})
density, err := client.Datasets.AirbnbMarketsNearby(ctx, crawlora.Params{"lat": 48.86, "lon": 2.35, "radius_m": 5000})
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

## Custom Retries And Observability

```go
client := crawlora.NewClient(
    crawlora.WithRetries(3),
    crawlora.WithMaxRetryDelay(10*time.Second),
    crawlora.WithRetryableStatuses(429, 503),                // or:
    crawlora.WithRetryPredicate(func(status int, err error) bool { return status >= 500 }),
    crawlora.WithOnRetry(func(attempt int, err error, delay time.Duration) {
        log.Printf("retry %d after %v: %v", attempt, delay, err)
    }),
    crawlora.WithRequestID(true), // generate x-request-id; available as (*Error).RequestID
    crawlora.WithLogger(func(event map[string]any) { log.Printf("%v", event) }),
)
```

Classify failures with `errors.Is(err, crawlora.ErrClient | ErrServer | ErrNetwork)`.

## Pagination

```go
// page/offset (auto-detected), per-page callback; return ErrStopPagination to stop early.
_ = client.Paginate(ctx, "ebay-seller-feedback", crawlora.Params{"seller": "acme"}, func(page any) error {
    return nil
})

// per-item iteration
_ = client.PaginateItems(ctx, "ebay-seller-feedback", crawlora.Params{"seller": "acme"}, func(item any) error {
    return nil
})

// cursor/token pagination
_ = client.Paginate(ctx, "producthunt-leaderboard", nil, func(page any) error { return nil },
    crawlora.WithCursorParam("cursor"),
    crawlora.WithNextCursor(func(page any) any {
        if m, ok := page.(map[string]any); ok {
            return m["next_cursor"]
        }
        return nil
    }),
)
```

## Streaming Responses

```go
out, err := client.Bing.Search(ctx, crawlora.Params{"q": "coffee"}, crawlora.WithResponseType(crawlora.ResponseStream))
if err != nil {
    return err
}
body := out.(io.ReadCloser)
defer body.Close()
// read body incrementally
```

## Environment Variables

`CRAWLORA_API_KEY` and `CRAWLORA_BASE_URL` are used when not set explicitly
(precedence: option > env > default).

## Middleware

```go
client := crawlora.NewClient(
    crawlora.WithBeforeRequest(func(req *http.Request) error {
        req.Header.Set("x-signature", sign(req))
        return nil
    }),
    crawlora.WithAfterResponse(func(operationID string, status int, headers http.Header, body any) (any, error) {
        return body, nil // return a replacement to transform
    }),
)
```

## Idempotency And Per-Request Retries

```go
client := crawlora.NewClient(crawlora.WithIdempotencyKeys(true)) // stable key on POST/PATCH retries

// override retry policy for one call
client.Bing.Search(ctx, crawlora.Params{"q": "coffee"},
    crawlora.WithRequestRetries(5),
    crawlora.WithRequestRetryPredicate(func(status int, err error) bool { return status >= 500 }),
)
```

## Rate Limiting

```go
client := crawlora.NewClient(
    crawlora.WithRateLimit(10),     // <= 10 requests/sec
    crawlora.WithMaxConcurrency(4), // <= 4 in flight
)
```

## Optional Live Smoke Tests

```sh
CRAWLORA_API_KEY=... go run ./examples/bing-search
CRAWLORA_API_KEY=... CRAWLORA_YOUTUBE_VIDEO_ID=... go run ./examples/youtube-transcript
```
