package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// PinResponse represents the Pinning Service API response format
type PinResponse struct {
	RequestID string   `json:"requestid"`
	Status    string   `json:"status"`
	Created   string   `json:"created"`
	Pin       PinInfo  `json:"pin"`
	Delegates []string `json:"delegates"`
}

// PinInfo represents the pin details within a PinResponse
type PinInfo struct {
	CID     string                 `json:"cid"`
	Name    string                 `json:"name,omitempty"`
	Origins []string               `json:"origins,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// PinListResponse represents the list pins API response
type PinListResponse struct {
	Count   int           `json:"count"`
	Results []PinResponse `json:"results"`
}

// UploadResponse represents the POST /v1/add response
type UploadResponse struct {
	CID  string `json:"cid"`
	Size int64  `json:"size"`
}

// NewPinCmd creates the pin command with subcommands
func NewPinCmd() *cobra.Command {
	pinCmd := &cobra.Command{
		Use:   "pin",
		Short: "Manage IPFS pins on Solvr",
		Long: `Manage IPFS pins on Solvr's pinning service.

Pin content to IPFS via Solvr's hosted infrastructure.
Supports pinning by CID or uploading files directly.

Subcommands:
  add       Pin a CID
  ls        List your pins
  status    Check pin status
  rm        Remove a pin
  add-file  Upload a file and pin it`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	pinCmd.AddCommand(newPinAddCmd())
	pinCmd.AddCommand(newPinLsCmd())
	pinCmd.AddCommand(newPinStatusCmd())
	pinCmd.AddCommand(newPinRmCmd())
	pinCmd.AddCommand(newPinAddFileCmd())

	return pinCmd
}

// loadPinConfig loads API key and URL from config/flags
func loadPinConfig(apiKey, apiURL string) (string, string, error) {
	config, err := loadConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}

	if apiKey == "" {
		if key, ok := config["api-key"]; ok {
			apiKey = key
		}
	}
	if apiKey == "" {
		return "", "", fmt.Errorf("API key not configured. Run 'solvr config set api-key <your-api-key>' first")
	}

	if apiURL == defaultAPIURL {
		if u, ok := config["api-url"]; ok {
			apiURL = u
		}
	}

	return apiKey, apiURL, nil
}

// doAuthRequest creates and executes an authenticated HTTP request
func doAuthRequest(method, requestURL, apiKey string, body io.Reader) (*http.Response, []byte, error) {
	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	if body != nil && method != "DELETE" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("failed to read response: %w", err)
	}

	return resp, respBody, nil
}

// checkAPIError returns an error if the response indicates a failure
func checkAPIError(resp *http.Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var apiErr APIError
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("API error: %s", apiErr.Error.Message)
	}
	return fmt.Errorf("API returned status %d", resp.StatusCode)
}

// --- pin add ---

func newPinAddCmd() *cobra.Command {
	var apiURL, apiKey, name string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "add <cid>",
		Short: "Pin a CID to IPFS via Solvr",
		Long: `Pin a CID to IPFS via Solvr's pinning service.

The CID must already exist on the IPFS network.

Examples:
  solvr pin add QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA
  solvr pin add QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA --name "my-file"
  solvr pin add bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cid := args[0]

			key, baseURL, err := loadPinConfig(apiKey, apiURL)
			if err != nil {
				return err
			}

			// Build request body
			reqBody := map[string]interface{}{
				"cid": cid,
			}
			if name != "" {
				reqBody["name"] = name
			}

			bodyBytes, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("failed to marshal request: %w", err)
			}

			resp, respBody, err := doAuthRequest("POST", baseURL+"/pins", key, bytes.NewReader(bodyBytes))
			if err != nil {
				return err
			}

			if err := checkAPIError(resp, respBody); err != nil {
				return err
			}

			var pinResp PinResponse
			if err := json.Unmarshal(respBody, &pinResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(pinResp)
			}

			displayPinResult(cmd, pinResp)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&name, "name", "", "Optional name for the pin")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON")

	return cmd
}

// --- pin ls ---

func newPinLsCmd() *cobra.Command {
	var apiURL, apiKey, statusFilter string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List your pins",
		Long: `List your pins on Solvr's IPFS pinning service.

Examples:
  solvr pin ls
  solvr pin ls --status pinned
  solvr pin ls --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, baseURL, err := loadPinConfig(apiKey, apiURL)
			if err != nil {
				return err
			}

			// Build URL with query params
			pinsURL, err := url.Parse(baseURL + "/pins")
			if err != nil {
				return fmt.Errorf("failed to parse URL: %w", err)
			}
			q := pinsURL.Query()
			if statusFilter != "" {
				q.Set("status", statusFilter)
			}
			pinsURL.RawQuery = q.Encode()

			resp, respBody, err := doAuthRequest("GET", pinsURL.String(), key, nil)
			if err != nil {
				return err
			}

			if err := checkAPIError(resp, respBody); err != nil {
				return err
			}

			var listResp PinListResponse
			if err := json.Unmarshal(respBody, &listResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(listResp)
			}

			displayPinList(cmd, listResp)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status: queued, pinning, pinned, failed")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON")

	return cmd
}

// --- pin status ---

func newPinStatusCmd() *cobra.Command {
	var apiURL, apiKey string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status <requestid>",
		Short: "Check the status of a pin",
		Long: `Check the status of a pin by its request ID.

Examples:
  solvr pin status req-123
  solvr pin status req-123 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := args[0]

			key, baseURL, err := loadPinConfig(apiKey, apiURL)
			if err != nil {
				return err
			}

			resp, respBody, err := doAuthRequest("GET", baseURL+"/pins/"+requestID, key, nil)
			if err != nil {
				return err
			}

			if err := checkAPIError(resp, respBody); err != nil {
				return err
			}

			var pinResp PinResponse
			if err := json.Unmarshal(respBody, &pinResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(pinResp)
			}

			displayPinResult(cmd, pinResp)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON")

	return cmd
}

// --- pin rm ---

func newPinRmCmd() *cobra.Command {
	var apiURL, apiKey string

	cmd := &cobra.Command{
		Use:   "rm <requestid>",
		Short: "Remove a pin",
		Long: `Remove a pin from Solvr's IPFS pinning service.

Examples:
  solvr pin rm req-123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := args[0]

			key, baseURL, err := loadPinConfig(apiKey, apiURL)
			if err != nil {
				return err
			}

			resp, respBody, err := doAuthRequest("DELETE", baseURL+"/pins/"+requestID, key, nil)
			if err != nil {
				return err
			}

			if err := checkAPIError(resp, respBody); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pin %s removed successfully.\n", requestID)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	return cmd
}

// --- pin add-file ---

func newPinAddFileCmd() *cobra.Command {
	var apiURL, apiKey, name string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "add-file <path>",
		Short: "Upload a file to IPFS and pin it",
		Long: `Upload a file to IPFS via Solvr and automatically pin it.

This is a convenience command that combines:
  1. POST /v1/add (upload file, get CID)
  2. POST /v1/pins (pin the CID)

Examples:
  solvr pin add-file ./data.json
  solvr pin add-file ./image.png --name "my-image"
  solvr pin add-file ./report.pdf --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			key, baseURL, err := loadPinConfig(apiKey, apiURL)
			if err != nil {
				return err
			}

			// Open the file
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			// Step 1: Upload to IPFS (POST /v1/add)
			cid, size, err := uploadFile(baseURL, key, file, filepath.Base(filePath))
			if err != nil {
				return err
			}

			// Step 2: Pin the CID (POST /v1/pins)
			pinName := name
			if pinName == "" {
				pinName = filepath.Base(filePath)
			}

			pinResp, err := pinCID(baseURL, key, cid, pinName)
			if err != nil {
				return err
			}

			if jsonOutput {
				result := map[string]interface{}{
					"cid":       cid,
					"size":      size,
					"requestid": pinResp.RequestID,
					"status":    pinResp.Status,
				}
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "File uploaded and pinned successfully!\n")
			fmt.Fprintf(out, "  CID:        %s\n", cid)
			fmt.Fprintf(out, "  Size:       %d bytes\n", size)
			fmt.Fprintf(out, "  Request ID: %s\n", pinResp.RequestID)
			fmt.Fprintf(out, "  Status:     %s\n", pinResp.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&name, "name", "", "Optional name for the pin")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON")

	return cmd
}

// uploadFile uploads a file via POST /v1/add and returns the CID and size
func uploadFile(baseURL, apiKey string, file *os.File, filename string) (string, int64, error) {
	// Create multipart body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", 0, fmt.Errorf("failed to write file to form: %w", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", baseURL+"/add", &buf)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read upload response: %w", err)
	}

	if err := checkAPIError(resp, respBody); err != nil {
		return "", 0, fmt.Errorf("upload failed: %w", err)
	}

	var uploadResp UploadResponse
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse upload response: %w", err)
	}

	return uploadResp.CID, uploadResp.Size, nil
}

// pinCID creates a pin via POST /v1/pins
func pinCID(baseURL, apiKey, cid, name string) (*PinResponse, error) {
	reqBody := map[string]interface{}{
		"cid": cid,
	}
	if name != "" {
		reqBody["name"] = name
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pin request: %w", err)
	}

	resp, respBody, err := doAuthRequest("POST", baseURL+"/pins", apiKey, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("pin failed: %w", err)
	}

	if err := checkAPIError(resp, respBody); err != nil {
		return nil, fmt.Errorf("pin failed: %w", err)
	}

	var pinResp PinResponse
	if err := json.Unmarshal(respBody, &pinResp); err != nil {
		return nil, fmt.Errorf("failed to parse pin response: %w", err)
	}

	return &pinResp, nil
}

// --- display helpers ---

func displayPinResult(cmd *cobra.Command, pin PinResponse) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Pin: %s\n", pin.RequestID)
	fmt.Fprintf(out, "  CID:    %s\n", pin.Pin.CID)
	fmt.Fprintf(out, "  Status: %s\n", pin.Status)
	if pin.Pin.Name != "" {
		fmt.Fprintf(out, "  Name:   %s\n", pin.Pin.Name)
	}
	fmt.Fprintf(out, "  Created: %s\n", pin.Created)
}

func displayPinList(cmd *cobra.Command, list PinListResponse) {
	out := cmd.OutOrStdout()

	if list.Count == 0 {
		fmt.Fprintln(out, "No pins found.")
		return
	}

	fmt.Fprintf(out, "%d pin(s) found:\n\n", list.Count)

	for _, pin := range list.Results {
		statusIcon := pinStatusIcon(pin.Status)
		name := pin.Pin.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Fprintf(out, "  %s %s  %s  [%s]\n", statusIcon, pin.RequestID, truncateCID(pin.Pin.CID), name)
	}
}

func pinStatusIcon(status string) string {
	switch status {
	case "pinned":
		return "[pinned]"
	case "queued":
		return "[queued]"
	case "pinning":
		return "[pinning]"
	case "failed":
		return "[failed]"
	default:
		return "[" + status + "]"
	}
}

func truncateCID(cid string) string {
	if len(cid) > 20 {
		return cid[:10] + "..." + cid[len(cid)-6:]
	}
	return cid
}
