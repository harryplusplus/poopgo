package main

import (
	"fmt"
	"os"

	"github.com/harryplusplus/poopgo/internal/app"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("POOPGO_API_KEY")
	apiBase := os.Getenv("POOPGO_BASE_URL")
	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}
	chatModel := os.Getenv("POOPGO_MODEL")
	if chatModel == "" {
		chatModel = "gpt-4o"
	}

	var initErr string
	if apiKey == "" {
		initErr = "POOPGO_API_KEY not set. Set it in your environment or .env file."
	}

	m := app.NewModel(apiKey, apiBase, chatModel, initErr)
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	m.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "poopgo: %v\n", err)
		os.Exit(1)
	}
}
