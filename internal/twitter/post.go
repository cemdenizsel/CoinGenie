package twitter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"cg-mentions-bot/internal/handlers"

	"github.com/hashicorp/go-retryablehttp"
)

// NewPoster returns a function that posts a reply tweet using Twitter API v2.
func NewPoster(baseURL, bearer string) func(ctx context.Context, in handlers.ReplyIn) error {
	client := retryablehttp.NewClient()
	client.Logger = nil

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
			return fmt.Errorf("twitter post failed: status %d", resp.StatusCode)
		}
		return nil
	}
}
