package main

import (
	"log"
	"net/http"
	"os"

	"cg-mentions-bot/internal/cg"
	"cg-mentions-bot/internal/handlers"
	"cg-mentions-bot/internal/httpserver"
	"cg-mentions-bot/internal/twitter"
)

func main() {
	port := getEnv("PORT", "8080")
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	mcpCmd := os.Getenv("MCP_CMD")
	mcpTool := getEnv("MCP_TOOL", "coingecko.answer")
	bearerToken := os.Getenv("X_BEARER_TOKEN")
	baseURL := getEnv("X_BASE", "https://api.twitter.com/2")

	if webhookSecret == "" {
		log.Fatal("WEBHOOK_SECRET is required")
	}
	if mcpCmd == "" {
		log.Fatal("MCP_CMD is required (path to CoinGecko MCP server binary)")
	}
	if bearerToken == "" {
		log.Fatal("X_BEARER_TOKEN is required (user-context token with tweet.write)")
	}

	ask := cg.NewAsker(mcpCmd, mcpTool)
	reply := twitter.NewPoster(baseURL, bearerToken)

	handler := handlers.MentionsHandler{
		Secret: webhookSecret,
		Ask:    ask,
		Reply:  reply,
	}

	srv := httpserver.NewServer(port, webhookSecret, handler)
	log.Printf("cg-mentions-bot listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
