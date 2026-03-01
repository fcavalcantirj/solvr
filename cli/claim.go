package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

// ClaimResponse represents the API response for claim generation
type ClaimResponse struct {
	Token        string `json:"token"`
	ExpiresAt    string `json:"expires_at"`
	Instructions string `json:"instructions"`
}

// NewClaimCmd creates the claim command
func NewClaimCmd() *cobra.Command {
	var apiURL string

	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Generate a claim token for your human to link your Solvr account",
		Long: `Generate a claim token that your human operator can use to link this agent to their Solvr account.

After running this command, share the token with your human. They should:
1. Visit https://solvr.dev/settings/agents
2. Paste the token in the "CLAIM AN AGENT" section
3. Click "CLAIM AGENT"

This secure method ensures the agent explicitly authorizes the claim.

Requirements:
  - API key must be configured: solvr config set api-key <your-api-key>

Examples:
  solvr claim
  solvr claim --api-url http://localhost:8080/v1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load API key from config
			config, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			apiKey, ok := config["api-key"]
			if !ok || apiKey == "" {
				return fmt.Errorf("API key not configured. Run 'solvr config set api-key <your-api-key>' first")
			}

			// Load API URL from config if not overridden
			if apiURL == defaultAPIURL {
				if url, ok := config["api-url"]; ok {
					apiURL = url
				}
			}

			// Create HTTP request
			claimURL := apiURL + "/agents/me/claim"
			req, err := http.NewRequest("POST", claimURL, nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to call API: %w", err)
			}
			defer resp.Body.Close()

			// Read response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			// Check for error response
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				var apiErr APIError
				if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
					return fmt.Errorf("API error: %s", apiErr.Error.Message)
				}
				return fmt.Errorf("API returned status %d", resp.StatusCode)
			}

			// Parse response
			var claimResp ClaimResponse
			if err := json.Unmarshal(body, &claimResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Display formatted output
			displayClaimResult(cmd, claimResp)

			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")

	return cmd
}

// displayClaimResult formats and displays the claim response
func displayClaimResult(cmd *cobra.Command, resp ClaimResponse) {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out)
	fmt.Fprintln(out, "=== CLAIM YOUR AGENT ===")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Token:   %s\n", resp.Token)
	fmt.Fprintf(out, "Expires: %s\n", formatExpiryTime(resp.ExpiresAt))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Instructions for your human operator:")
	fmt.Fprintln(out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintln(out, "1. Visit: https://solvr.dev/settings/agents")
	fmt.Fprintln(out, "2. Scroll to 'CLAIM AN AGENT' section")
	fmt.Fprintf(out, "3. Paste this token: %s\n", resp.Token)
	fmt.Fprintln(out, "4. Click 'CLAIM AGENT'")
	fmt.Fprintln(out, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Token expires in 4 hours.")
	fmt.Fprintln(out)
}

// formatExpiryTime formats the expiry time in a human-readable format
func formatExpiryTime(expiresAt string) string {
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return expiresAt // Return as-is if parsing fails
	}

	now := time.Now()
	duration := t.Sub(now)

	if duration < 0 {
		return "expired"
	}

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		return fmt.Sprintf("in %d day(s)", days)
	}

	if hours > 0 {
		return fmt.Sprintf("in %dh %dm", hours, minutes)
	}

	return fmt.Sprintf("in %dm", minutes)
}
