# Crawlora Go SDK

Git-only beta SDK for the public Crawlora API.

## Install

```sh
go get github.com/Crawlora-org/crawlora-go-sdk@v1.2.0-sdk.2
```

## Usage

```go
client := crawlora.NewClient(crawlora.WithAPIKey("..."))
result, err := client.Bing.Search(ctx, crawlora.Params{"q": "coffee shops", "count": 10})
```

## Configuration

```go
client := crawlora.NewClient(
    crawlora.WithAPIKey(os.Getenv("CRAWLORA_API_KEY")),
    crawlora.WithBaseURL("https://api.crawlora.net/api/v1"),
    crawlora.WithRetries(2),
)
```

Per-request options are available for custom headers, timeout, and response
mode:

```go
text, err := client.YouTube.Transcript(ctx, crawlora.Params{
    "id": "VIDEO_ID",
    "format": "text",
}, crawlora.WithResponseType(crawlora.ResponseText))
```

API failures return `*crawlora.Error` with HTTP status, API code, parsed body,
raw body, and the underlying transport error when one exists.
