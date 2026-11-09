package covers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Package covers draws a blog post's cover image (SEO audit 2026-11-08, Release A). It is its
// own package because handlers cannot import services (services imports handlers).
//
// Every blog page previewed
// with the one site card; a drawn card (frontend lib/seo/og-card.tsx) is the fallback now,
// and this service draws a real cover once per post with OpenAI's image model, from the
// post's title, excerpt and tags, in the site's look. The bytes are stored by
// db.BlogCoverRepository and served at GET /v1/blog/{slug}/cover.jpg.

// The defaults of the image service.
const (
	DefaultOpenAIBaseURL = "https://api.openai.com/v1"
	// DefaultModel is the newest image model the key lists (checked 2026-11-08).
	DefaultModel = "gpt-image-2"
	// DefaultSize is the model's landscape size, the closest to a 1200×630 preview.
	DefaultSize = "1536x1024"
	// DefaultJPEGQuality keeps a cover near 200 KB, light enough for a page's hero.
	DefaultJPEGQuality = 85
	// DefaultTimeout covers a high-quality render, which took 59 s in the first test.
	DefaultTimeout = 3 * time.Minute

	titleMax   = 300
	excerptMax = 600
	tagsMax    = 6
)

// The errors a Generate can answer.
var (
	ErrMissingAPIKey = errors.New("cover: OPENAI_API_KEY must not be empty")
	ErrEmptyBrief    = errors.New("cover: the brief needs at least a title")
	ErrNoImage       = errors.New("cover: the API answered no image")
)

// Brief is what the cover is drawn from.
type Brief struct {
	Title   string
	Excerpt string
	Tags    []string
}

// Image is a drawn cover and what drew it.
type Image struct {
	ContentType string
	Model       string
	Prompt      string
	Bytes       []byte
}

// Service draws a cover for a brief.
type Service interface {
	Generate(ctx context.Context, brief Brief) (*Image, error)
}

// OpenAIService draws covers with OpenAI's images API.
type OpenAIService struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	model      string
}

// Option configures the service.
type Option func(*OpenAIService)

// WithBaseURL points the service at another API root (tests).
func WithBaseURL(url string) Option {
	return func(s *OpenAIService) { s.baseURL = strings.TrimRight(url, "/") }
}

// WithModel picks the image model.
func WithModel(model string) Option {
	return func(s *OpenAIService) {
		if model != "" {
			s.model = model
		}
	}
}

// NewOpenAIService creates the service. A nil option is ignored.
func NewOpenAIService(apiKey string, opts ...Option) *OpenAIService {
	s := &OpenAIService{apiKey: apiKey, baseURL: DefaultOpenAIBaseURL, model: DefaultModel,
		httpClient: &http.Client{Timeout: DefaultTimeout}}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// BuildPrompt writes the brief the model draws from: the site's look (paper and ink,
// one muted amber accent), an abstract editorial composition about the post, and nothing a
// model gets wrong: no text, no logos, no people.
func BuildPrompt(brief Brief) string {
	title := cut(strings.TrimSpace(brief.Title), titleMax)
	var b strings.Builder
	b.WriteString("Editorial cover illustration for a developer blog post titled '")
	b.WriteString(title)
	b.WriteString("'.")
	if tags := cleanTags(brief.Tags); len(tags) > 0 {
		b.WriteString(" Topics: ")
		b.WriteString(strings.Join(tags, ", "))
		b.WriteString(".")
	}
	if excerpt := cut(strings.TrimSpace(brief.Excerpt), excerptMax); excerpt != "" {
		b.WriteString(" What the post says: ")
		b.WriteString(excerpt)
	}
	b.WriteString(" Style: minimal, flat, warm off-white paper background (#f7f5f0), near-black ink (#111111), one accent of muted amber.")
	b.WriteString(" An abstract composition that suggests the subject with clean geometric shapes and thin lines, generous whitespace.")
	b.WriteString(" No text, no letters, no logos, no people, no screenshots. Landscape, suitable as a wide blog header.")
	return b.String()
}

type request struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	Size              string `json:"size"`
	Quality           string `json:"quality"`
	OutputFormat      string `json:"output_format"`
	OutputCompression int    `json:"output_compression"`
	N                 int    `json:"n"`
}

type response struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Data []struct {
		B64 string `json:"b64_json"`
	} `json:"data"`
}

// Generate draws one cover.
func (s *OpenAIService) Generate(ctx context.Context, brief Brief) (*Image, error) {
	if s.apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if strings.TrimSpace(brief.Title) == "" {
		return nil, ErrEmptyBrief
	}
	prompt := BuildPrompt(brief)
	body, err := json.Marshal(request{Model: s.model, Prompt: prompt, Size: DefaultSize, Quality: "high",
		OutputFormat: "jpeg", OutputCompression: DefaultJPEGQuality, N: 1})
	if err != nil {
		return nil, fmt.Errorf("cover: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cover: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cover: request failed: %w", err)
	}
	defer res.Body.Close() //nolint:errcheck // a response body
	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("cover: read response: %w", err)
	}
	var out response
	if jsonErr := json.Unmarshal(raw, &out); jsonErr != nil && res.StatusCode == http.StatusOK {
		return nil, fmt.Errorf("cover: parse response: %w", jsonErr)
	}
	if res.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(raw))
		if out.Error != nil {
			msg = out.Error.Message
		}
		return nil, fmt.Errorf("cover: API answered %d: %s", res.StatusCode, cut(msg, 300))
	}
	if len(out.Data) == 0 || out.Data[0].B64 == "" {
		return nil, ErrNoImage
	}
	img, err := base64.StdEncoding.DecodeString(out.Data[0].B64)
	if err != nil {
		return nil, fmt.Errorf("cover: decode image: %w", err)
	}
	if len(img) < 4 || img[0] != 0xFF || img[1] != 0xD8 {
		return nil, fmt.Errorf("%w: not a JPEG", ErrNoImage)
	}
	return &Image{Bytes: img, ContentType: "image/jpeg", Model: s.model, Prompt: prompt}, nil
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
		if len(out) == tagsMax {
			break
		}
	}
	return out
}

func cut(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}
