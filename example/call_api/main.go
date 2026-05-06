package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/shayne/stock-ticker-stream-deck-plugin/pkg/api"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("usage: %s <provider:finnhub|yahoo> <symbol> [finnhub-api-key]", os.Args[0])
	}

	provider := os.Args[1]
	symbol := os.Args[2]
	if len(os.Args) > 3 {
		api.SetAPIKey(os.Args[3])
	}

	result, err := api.GetQuote(symbol, provider)
	if err != nil {
		log.Fatal(err)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	_, _ = os.Stdout.Write(output)
}
