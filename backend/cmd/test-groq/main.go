package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fcavalcantirj/solvr/internal/services"
)

func main() {
	apiKey := os.Getenv("TEST_GROQ_API_KEY")
	if apiKey == "" {
		fmt.Println("TEST_GROQ_API_KEY not set")
		os.Exit(1)
	}

	ctx := context.Background()

	// --- Test 1: Moderation on English post ---
	fmt.Println("=== TEST 1: Moderation (English, should APPROVE) ===")
	modSvc := services.NewContentModerationService(apiKey)
	r1, err := modSvc.ModerateContent(ctx, services.ModerationInput{
		Title:       "How to handle goroutine leaks in Go",
		Description: "I have a service that spawns goroutines and I suspect some are leaking. How do I detect and fix goroutine leaks?",
		Tags:        []string{"go", "concurrency"},
	})
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  approved=%v language=%q reasons=%v\n", r1.Approved, r1.LanguageDetected, r1.RejectionReasons)
	}

	// --- Test 2: Moderation on Portuguese post ---
	fmt.Println("\n=== TEST 2: Moderation (Portuguese, should REJECT) ===")
	r2, err := modSvc.ModerateContent(ctx, services.ModerationInput{
		Title:       "Como resolver vazamentos de goroutine em Go",
		Description: "Tenho um serviço que cria goroutines e suspeito que algumas estão vazando. Como detectar e corrigir vazamentos de goroutine?",
		Tags:        []string{"go", "concorrência"},
	})
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  approved=%v language=%q reasons=%v\n", r2.Approved, r2.LanguageDetected, r2.RejectionReasons)
		fmt.Printf("  explanation=%q\n", r2.Explanation)
	}

	// --- Test 3: Translation of the Portuguese post ---
	fmt.Println("\n=== TEST 3: Translation (Portuguese → English) ===")
	transSvc := services.NewTranslationService(apiKey)
	r3, err := transSvc.TranslateContent(ctx, services.TranslationInput{
		Title:       "Como resolver vazamentos de goroutine em Go",
		Description: "Tenho um serviço que cria goroutines e suspeito que algumas estão vazando. Como detectar e corrigir vazamentos de goroutine?",
		Language:    "Portuguese",
	})
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  title:       %q\n", r3.Title)
		fmt.Printf("  description: %q\n", r3.Description)
	}

	// --- Test 4: Moderation on the translated post ---
	if r3 != nil {
		fmt.Println("\n=== TEST 4: Moderation on TRANSLATED post (should APPROVE) ===")
		r4, err := modSvc.ModerateContent(ctx, services.ModerationInput{
			Title:       r3.Title,
			Description: r3.Description,
			Tags:        []string{"go", "concurrency"},
		})
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			fmt.Printf("  approved=%v language=%q reasons=%v\n", r4.Approved, r4.LanguageDetected, r4.RejectionReasons)
		}
	}
}
