package twitter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"cg-mentions-bot/internal/handlers"

	"github.com/dghubble/oauth1"
	"github.com/hashicorp/go-retryablehttp"
)

// NewPoster returns a function that posts a reply tweet using Twitter API v2.
// Auth modes:
// - Default (OAuth2 bearer): set X_BEARER_TOKEN
// - OAuth1: set X_AUTH_MODE=oauth1 and provide X_CONSUMER_KEY, X_CONSUMER_SECRET, X_ACCESS_TOKEN, X_ACCESS_SECRET
func NewPoster(baseURL, bearer string) func(ctx context.Context, in handlers.ReplyIn) error {
	client := retryablehttp.NewClient()
	client.Logger = nil

	useOAuth1 := strings.EqualFold(os.Getenv("X_AUTH_MODE"), "oauth1")
	var oauth1Client *http.Client
	if useOAuth1 {
		ck := os.Getenv("X_CONSUMER_KEY")
		cs := os.Getenv("X_CONSUMER_SECRET")
		at := os.Getenv("X_ACCESS_TOKEN")
		as := os.Getenv("X_ACCESS_SECRET")
		config := oauth1.NewConfig(ck, cs)
		token := oauth1.NewToken(at, as)
		oauth1Client = config.Client(context.Background(), token)
	}

	return func(ctx context.Context, in handlers.ReplyIn) error {
		url := fmt.Sprintf("%s/tweets", baseURL)
		body := map[string]any{
			"text": in.Text,
			"reply": map[string]any{
				"in_reply_to_tweet_id": in.InReplyTo,
			},
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}

		if useOAuth1 {
			// Use raw http.Client with OAuth1 transport
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := oauth1Client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 300 {
				b, _ := io.ReadAll(resp.Body)
				if len(b) > 0 {
					return fmt.Errorf("twitter post failed: status %d: %s", resp.StatusCode, string(b))
				}
				return fmt.Errorf("twitter post failed: status %d", resp.StatusCode)
			}
			return nil
		}

		// OAuth2 bearer default
		req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+bearer)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			if len(b) > 0 {
				return fmt.Errorf("twitter post failed: status %d: %s", resp.StatusCode, string(b))
			}
			return fmt.Errorf("twitter post failed: status %d", resp.StatusCode)
		}
		return nil
	}
}
