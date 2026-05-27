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

	videoID := os.Getenv("CRAWLORA_YOUTUBE_VIDEO_ID")
	if len(os.Args) > 1 {
		videoID = os.Args[1]
	}
	if videoID == "" {
		fmt.Fprintln(os.Stderr, "set CRAWLORA_YOUTUBE_VIDEO_ID or pass a video id to run this live example")
		return
	}

	format := "text"
	result, err := client.YouTube.TranscriptTyped(
		context.Background(),
		crawlora.YouTubeTranscriptParams{Id: videoID, Format: &format},
		crawlora.WithResponseType(crawlora.ResponseText),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
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
