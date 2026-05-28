package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	crawlora "github.com/Crawlora-org/crawlora-go-sdk"
)

func main() {
	client, ok := newClient()
	if !ok {
		return
	}

	result, err := client.Bing.SearchTyped(context.Background(), crawlora.BingSearchParams{
		Q:     "coffee shops",
		Count: crawlora.Int(5),
	})
	if err != nil {
		log.Fatal(err)
	}
	printJSON(result)
}

func newClient() (*crawlora.Client, bool) {
	apiKey := os.Getenv("CRAWLORA_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "set CRAWLORA_API_KEY to run this live example")
		return nil, false
	}
	opts := []crawlora.Option{crawlora.WithAPIKey(apiKey)}
	if baseURL := os.Getenv("CRAWLORA_BASE_URL"); baseURL != "" {
		opts = append(opts, crawlora.WithBaseURL(baseURL))
	}
	return crawlora.NewClient(opts...), true
}

func printJSON(value any) {
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}
