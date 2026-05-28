# Crawlora Go SDK

Go client for the public Crawlora API. Use it to call Crawlora scraping,
search, marketplace, media, maps, finance, and usage endpoints with generated
service groups, typed parameter structs, operation constants, and typed response
aliases.

- Runtime: Go 1.22+
- Auth: `x-api-key`
- Default API base URL: `https://api.crawlora.net/api/v1`
- Reference: [operations](docs/operations.md) and [recipes](docs/recipes.md)

## Install

Install the module from Git:

```sh
go get github.com/Crawlora-org/crawlora-go-sdk@latest
```

For reproducible builds, pin a released tag:

```sh
go get github.com/Crawlora-org/crawlora-go-sdk@TAG
```

## API Key

Create or sign in to your Crawlora account at [crawlora.net](https://crawlora.net),
then create an API key in the dashboard.

```sh
read -r CRAWLORA_API_KEY
export CRAWLORA_API_KEY
```

## First Request

```go
package main

import (
	"context"
	"fmt"
	"os"

	crawlora "github.com/Crawlora-org/crawlora-go-sdk"
)

func main() {
	client := crawlora.NewClient(
		crawlora.WithAPIKey(os.Getenv("CRAWLORA_API_KEY")),
	)

	response, err := client.Bing.Search(context.Background(), crawlora.Params{
		"q":     "coffee shops",
		"count": 10,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("%#v\n", response)
}
```

Endpoint groups are generated from the public API contract, so common calls are
available as methods such as `client.Bing.Search(...)`,
`client.YouTube.Transcript(...)`, and `client.Google.MapSearch(...)`.

## Typed Calls

Typed endpoint variants are generated for every operation:

```go
response, err := client.Bing.SearchTyped(ctx, crawlora.BingSearchParams{
	Q:     "coffee shops",
	Count: crawlora.Int(10),
})
```

Optional scalar fields use pointer helpers such as `crawlora.String(...)`,
`crawlora.Int(...)`, `crawlora.Bool(...)`, and `crawlora.Float64(...)`.

You can also call by operation id with generated constants and typed response
decoding:

```go
response, err := crawlora.RequestTyped[crawlora.BingSearchResponse](
	client,
	ctx,
	crawlora.OperationBingSearch,
	crawlora.Params{"q": "coffee shops"},
)
```

## Configuration

```go
client := crawlora.NewClient(
	crawlora.WithAPIKey(os.Getenv("CRAWLORA_API_KEY")),
	crawlora.WithBaseURL("https://api.crawlora.net/api/v1"),
	crawlora.WithRetries(2),
	crawlora.WithRetryDelay(250*time.Millisecond),
	crawlora.WithHeader("x-client", "my-app"),
)
```

Per-request options can override headers, timeout, and response mode:

```go
response, err := client.Bing.Search(
	ctx,
	crawlora.Params{"q": "coffee shops"},
	crawlora.WithRequestTimeout(10*time.Second),
	crawlora.WithRequestHeader("x-request-id", "search-001"),
)
```

## Text Responses

Most endpoints return JSON. Endpoints that support alternate text output, such
as YouTube transcripts, can opt into text mode:

```go
transcript, err := client.YouTube.Transcript(
	ctx,
	crawlora.Params{
		"id":     "VIDEO_ID",
		"format": "text",
	},
	crawlora.WithResponseType(crawlora.ResponseText),
)
```

## Errors

Failed API calls return `*crawlora.Error`:

```go
var apiErr *crawlora.Error
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Status, apiErr.Code, apiErr.Body)
}
```

The error includes HTTP `Status`, optional API `Code`, parsed `Body`, `RawBody`,
and the underlying parser or transport error when available.

## Examples

Runnable examples live under `examples/` and skip cleanly when required
environment variables are missing:

```sh
go run ./examples/bing-search
go run ./examples/youtube-transcript
```

Set `CRAWLORA_BASE_URL` to point examples at a staging or local API.

## Module Notes

Go consumers use this repository directly as a Go module:

```sh
go get github.com/Crawlora-org/crawlora-go-sdk@latest
```

Pin an explicit released tag for production applications and upgrade
intentionally.
