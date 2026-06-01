package main

import (
	"context"
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

	seller := os.Getenv("CRAWLORA_EBAY_SELLER")
	if seller == "" {
		seller = "garlandcomputer"
	}
	count := 0
	err := client.PaginateItems(context.Background(), "ebay-seller-feedback",
		crawlora.Params{"seller": seller},
		func(item any) error {
			count++
			return nil
		},
		crawlora.WithMaxPages(3),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("collected %d feedback items across up to 3 pages\n", count)
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
