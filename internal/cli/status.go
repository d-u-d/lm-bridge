package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"lm-bridge/internal/db"
	"lm-bridge/internal/llm"
)

func runStatus(_ []string) {
	store, _ := db.Open()

	provider := "lmstudio"
	var lmURL, model string
	if store != nil {
		if p, _ := store.GetSetting("provider"); p != "" {
			provider = p
		}
		lmURL, _ = store.GetSetting("lmstudio_url")
		model, _ = store.GetSetting("openrouter_model")
	}

	fmt.Printf("Provider:  %s\n", provider)

	if provider == "lmstudio" {
		url := llm.DefaultBaseURL
		if lmURL != "" {
			url = lmURL
		}
		fmt.Printf("URL:       %s\n", url)
	}

	client := newClient(store)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	online, detectedModel, _ := client.ModelStatus(ctx)

	if detectedModel != "" {
		fmt.Printf("Model:     %s\n", detectedModel)
	} else if model != "" {
		fmt.Printf("Model:     %s\n", model)
	}

	if online {
		fmt.Println("Status:    ✓ ready")
	} else {
		if provider == "lmstudio" {
			fmt.Println("Status:    ✗ offline — LM Studio is not running")
		} else {
			fmt.Println("Status:    ✗ offline — API key not configured")
		}
		os.Exit(1)
	}
}
