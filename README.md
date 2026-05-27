# Crawlora Go SDK

Git-installable beta SDK for the public Crawlora API.

## Install

```sh
go get github.com/Crawlora-org/crawlora-go-sdk@v1.2.0-sdk.4
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "os"

    crawlora "github.com/Crawlora-org/crawlora-go-sdk"
)

func main() {
    client := crawlora.NewClient(crawlora.WithAPIKey(os.Getenv("CRAWLORA_API_KEY")))
    result, err := client.Bing.Search(context.Background(), crawlora.Params{
        "q": "coffee shops",
        "count": 10,
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("%#v\n", result)
}
```

Typed endpoint variants are generated for every operation:

```go
count := 10
result, err := client.Bing.SearchTyped(ctx, crawlora.BingSearchParams{
    Q:     "coffee shops",
    Count: &count,
})
```

## Configuration

```go
client := crawlora.NewClient(
    crawlora.WithAPIKey(os.Getenv("CRAWLORA_API_KEY")),
    crawlora.WithBaseURL("https://api.crawlora.net/api/v1"),
    crawlora.WithRetries(2),
    crawlora.WithRetryDelay(250*time.Millisecond),
)
```

Per-request options are available for custom headers, timeout, and response
mode:

```go
text, err := client.YouTube.Transcript(ctx, crawlora.Params{
    "id": "VIDEO_ID",
    "format": "text",
}, crawlora.WithResponseType(crawlora.ResponseText), crawlora.WithRequestTimeout(10*time.Second))
```

Multipart upload endpoints accept byte slices, file paths, or `io.Reader`
values:

```go
result, err := client.Google.Lens(ctx, crawlora.Params{"image": []byte("image-bytes")})
```

API failures return `*crawlora.Error` with HTTP status, API code, parsed body,
raw body, and the underlying transport error when one exists.

## Versioning

This SDK is currently released as Git beta tags. Pin an explicit tag in
applications and upgrade intentionally.

## Regeneration

The committed `openapi/public.json` is the SDK contract source. Regenerate after
updating that file:

```sh
python3 scripts/generate.py
gofmt -w operations_generated.go
go test ./...
```

## Optional Live Smoke Test

Default tests use local mock servers. For live API checks, set
`CRAWLORA_API_KEY` in your own environment and call a low-cost endpoint manually.
Live calls are not part of default CI.
