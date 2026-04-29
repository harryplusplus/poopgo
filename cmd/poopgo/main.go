package main

import (
	"fmt"
	"os"

	"github.com/harryplusplus/poopgo/internal/app"
	tea "charm.land/bubbletea/v2"
)

func main() {
	apiKey := os.Getenv("POOPGO_API_KEY")
	baseURL := os.Getenv("POOPGO_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	chatModel := os.Getenv("POOPGO_MODEL")
	if chatModel == "" {
		chatModel = "gpt-4o"
	}

	// Provider selection: POOPGO_PROVIDER=fake → fake (no API calls),
	// anything else → real HTTP API.
	provider := os.Getenv("POOPGO_PROVIDER")
	var streamProvider app.StreamProvider
	if provider == "fake" {
		streamProvider = app.NewFakeProvider()
	} else {
		streamProvider = app.NewRealProvider(apiKey, baseURL)
	}

	if apiKey == "" && provider != "fake" {
		fmt.Fprintf(os.Stderr, "poopgo: POOPGO_API_KEY not set. Set it in your environment.\n")
		os.Exit(1)
	}

	reasoningEffort := os.Getenv("POOPGO_REASONING_EFFORT")
	temperature := os.Getenv("POOPGO_TEMPERATURE")

	m := app.NewModel(apiKey, baseURL, chatModel, reasoningEffort, temperature, streamProvider)
	p := tea.NewProgram(
		m,
	)
	m.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "poopgo: %v\n", err)
		os.Exit(1)
	}
}
