# Crawlora Go SDK

Git-installable beta SDK for the public Crawlora API.

Website: [crawlora.net](https://crawlora.net)

## Install

```sh
go get github.com/Crawlora-org/crawlora-go-sdk@latest
```

For reproducible builds, pin the current beta release tag:

```sh
go get github.com/Crawlora-org/crawlora-go-sdk@v1.2.0-sdk.9
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

API failures return `*crawlora.Error` with HTTP status, API code, parsed body,
raw body, and the underlying transport error when one exists.

## Examples

Runnable examples live under `examples/`:

```sh
CRAWLORA_API_KEY=... go run ./examples/bing-search
CRAWLORA_API_KEY=... CRAWLORA_YOUTUBE_VIDEO_ID=... go run ./examples/youtube-transcript
```

Each example also accepts `CRAWLORA_BASE_URL` for staging or local API testing.
The examples exit without making a request when the required live environment
variables are not set.

## Versioning

This SDK is currently released as Git beta tags. The moving `latest` tag tracks
the current promoted beta, while explicit tags such as `v1.2.0-sdk.9` remain
available for reproducible builds. Pin an explicit tag in production
applications and upgrade intentionally.

## Registry Readiness

Go consumers use this repository directly as a Go module:

```sh
go get github.com/Crawlora-org/crawlora-go-sdk@latest
```

The module path is stable for Go package discovery. Future releases should keep
`main`, the current beta tag, and the moving `latest` tag aligned with the
promoted SDK commit.

## Optional Live Smoke Test

Default tests use local mock servers. The programs under `examples/` can be used
as optional live smoke tests when `CRAWLORA_API_KEY` is available. Live calls are
not part of default CI.
