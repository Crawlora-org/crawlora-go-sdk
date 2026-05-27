# Crawlora Go SDK

Git-only beta SDK for the public Crawlora API.

## Install

```sh
go get github.com/crawlora/crawlora-go-sdk@v1.2.0-sdk.1
```

## Usage

```go
client := crawlora.NewClient(crawlora.WithAPIKey("..."))
result, err := client.Bing.Search(ctx, crawlora.Params{"q": "coffee shops", "count": 10})
```
