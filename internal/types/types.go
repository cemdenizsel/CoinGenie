package types

// Mention represents one mention payload item.
type Mention struct {
	TweetID        string `json:"tweet_id"`
	Text           string `json:"text"`
	AuthorID       string `json:"author_id"`
	AuthorUsername string `json:"author_username"`
	ConversationID string `json:"conversation_id"`
	CreatedAt      string `json:"created_at"`
}

// MentionsPayload is the full body we receive from n8n.
type MentionsPayload struct {
	Count    int            `json:"count"`
	Mentions []Mention      `json:"mentions"`
	Meta     map[string]any `json:"meta,omitempty"`
}
