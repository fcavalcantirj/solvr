package covers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A blog post's cover (SEO audit 2026-11-08, Release A): drawn once from the title, the
// excerpt and the tags by OpenAI's image model, as a JPEG the page and the link preview can
// carry. The prompt asks for the site's look and for no text, logos or people; the service
// asks for a landscape JPEG and hands back the bytes with what drew them.
func TestBuildPrompt(t *testing.T) {
	p := BuildPrompt(Brief{
		Title:   "What AI provider APIs actually expose for usage monitoring",
		Excerpt: "If you want to build a dashboard showing how much of your AI credits you have left…",
		Tags:    []string{"anthropic", "openai", "usage-monitoring", "", "api"},
	})
	assert.Contains(t, p, "What AI provider APIs actually expose for usage monitoring")
	assert.Contains(t, p, "anthropic, openai, usage-monitoring, api")
	assert.Contains(t, p, "dashboard showing how much")
	for _, want := range []string{"no text", "no logos", "no people", "#f7f5f0", "landscape"} {
		assert.Contains(t, strings.ToLower(p), want, want)
	}
	assert.Less(t, len(BuildPrompt(Brief{Title: strings.Repeat("x", 2000), Excerpt: strings.Repeat("y", 5000)})), 2400, "the brief is cut, not sent whole")
}

func TestOpenAIService_GeneratesAJPEGCover(t *testing.T) {
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'f', 'a', 'k', 'e'}
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(jpeg)}},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	svc := NewOpenAIService("sk-test", WithBaseURL(srv.URL+"/v1"))
	img, err := svc.Generate(context.Background(), Brief{Title: "Nine traps shipping a deep-sleeping board", Tags: []string{"esp32"}})
	require.NoError(t, err)
	assert.Equal(t, "Bearer sk-test", auth)
	assert.Equal(t, DefaultModel, got["model"])
	assert.Equal(t, "1536x1024", got["size"])
	assert.Equal(t, "high", got["quality"])
	assert.Equal(t, "jpeg", got["output_format"])
	assert.Equal(t, float64(DefaultJPEGQuality), got["output_compression"])
	assert.Contains(t, got["prompt"], "Nine traps shipping a deep-sleeping board")
	assert.Equal(t, jpeg, img.Bytes)
	assert.Equal(t, "image/jpeg", img.ContentType)
	assert.Equal(t, DefaultModel, img.Model)
	assert.Equal(t, got["prompt"], img.Prompt)
}

func TestOpenAIService_RefusesWhatIsNotAnImage(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"an API error": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"error":{"message":"rate limited"}}`)); err != nil {
				t.Error(err)
			}
		},
		"no data": func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write([]byte(`{"data":[]}`)); err != nil {
				t.Error(err)
			}
		},
		"not a JPEG": func(w http.ResponseWriter, _ *http.Request) {
			if err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString([]byte("<html>"))}}}); err != nil {
				t.Error(err)
			}
		},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			_, err := NewOpenAIService("sk-test", WithBaseURL(srv.URL+"/v1")).Generate(context.Background(), Brief{Title: "T"})
			require.Error(t, err)
		})
	}
	_, err := NewOpenAIService("", nil).Generate(context.Background(), Brief{Title: "T"})
	assert.ErrorIs(t, err, ErrMissingAPIKey)
	_, err = NewOpenAIService("sk-test").Generate(context.Background(), Brief{})
	assert.ErrorIs(t, err, ErrEmptyBrief)
}
