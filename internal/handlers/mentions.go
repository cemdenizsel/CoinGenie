package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"cg-mentions-bot/internal/types"
)

// MentionsHandler handles POST /mentions events.
type MentionsHandler struct {
	Secret string
	Ask    func(ctx context.Context, text string) (string, error)
	Reply  func(ctx context.Context, in ReplyIn) error
	// If set, uses the agent binary to both answer and post per mention.
	AgentRun func(ctx context.Context, question string, replyTo string) (string, error)
}

// ReplyIn contains minimal info to reply to a tweet.
type ReplyIn struct {
	InReplyTo string
	Text      string
}

// Handle verifies secret (if configured), processes mentions, and returns a summary.
func (h MentionsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if h.Secret != "" && r.Header.Get("X-Webhook-Secret") != h.Secret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Accept either a single payload object or an array of payloads
	var payload types.MentionsPayload
	var payloads []types.MentionsPayload
	if err := json.Unmarshal(body, &payloads); err != nil {
		// Not an array, try single
		if err2 := json.Unmarshal(body, &payload); err2 != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		payloads = []types.MentionsPayload{payload}
	}

	// Flatten mentions
	mentions := make([]types.Mention, 0)
	received := 0
	for _, p := range payloads {
		if p.Count > 0 {
			received += p.Count
		}
		if len(p.Mentions) > 0 {
			mentions = append(mentions, p.Mentions...)
		}
	}
	if received == 0 {
		received = len(mentions)
	}

	type res struct {
		TweetID string `json:"tweet_id"`
		Posted  bool   `json:"posted"`
		Error   string `json:"error,omitempty"`
	}

	results := make([]res, 0, len(mentions))

	for _, m := range mentions {
		q := normalizeTweetText(m.Text)
		if h.AgentRun != nil {
			if _, err := h.AgentRun(r.Context(), q, m.TweetID); err != nil {
				results = append(results, res{TweetID: m.TweetID, Posted: false, Error: err.Error()})
			} else {
				results = append(results, res{TweetID: m.TweetID, Posted: true})
			}
			continue
		}

		ans, err := h.Ask(r.Context(), q)
		if err != nil {
			results = append(results, res{TweetID: m.TweetID, Posted: false, Error: err.Error()})
			continue
		}

		if postErr := h.Reply(r.Context(), ReplyIn{InReplyTo: m.TweetID, Text: ans}); postErr != nil {
			results = append(results, res{TweetID: m.TweetID, Posted: false, Error: postErr.Error()})
			continue
		}

		results = append(results, res{TweetID: m.TweetID, Posted: true})
	}

	summary := struct {
		Received  int   `json:"received"`
		Processed int   `json:"processed"`
		Results   []res `json:"results"`
	}{
		Received:  received,
		Processed: len(mentions),
		Results:   results,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(summary)
}

// normalizeTweetText removes handles and URLs and trims whitespace to form a concise question input.
func normalizeTweetText(s string) string {
	// Remove URLs
	urlRe := regexp.MustCompile(`https?://\S+`)
	s = urlRe.ReplaceAllString(s, " ")
	// Remove @handles
	handleRe := regexp.MustCompile(`@[A-Za-z0-9_]+`)
	s = handleRe.ReplaceAllString(s, " ")
	// Collapse whitespace
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	return s
}
