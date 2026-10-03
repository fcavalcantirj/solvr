package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const inlineInputs = `{
  "label": "test",
  "price_source": "hypothetical placeholder",
  "capacity": {
    "monthly_active_participants": {"value": 1000000, "source": "hypothetical"},
    "daily_active_share": {"value": 0.1, "source": "hypothetical"},
    "requests_per_daily_participant": {"value": 20, "source": "hypothetical"},
    "burst_factor": {"value": 2, "source": "observed"},
    "measured_capacity_rps": {"value": 100, "source": "observed"}
  },
  "cost": {
    "prices": {"compute_per_month": 50, "database_per_month": 30, "storage_per_gb_month": 0.1,
      "egress_per_gb": 0.05, "observability_per_month": 10, "observability_per_gb": 0.5,
      "embedding_per_million_tokens": 0.18, "email_per_1000": 1},
    "usage": {"activated_rooms": {"value": 4, "source": "observed"}, "entries": {"value": 1515, "source": "observed"},
      "retained_gb": {"value": 0.085, "source": "observed"}, "searches": {"value": 106, "source": "observed"},
      "returning_participants": {"value": 1, "source": "observed"}, "egress_gb": {"value": 5, "source": "hypothetical"},
      "observability_gb": {"value": 1, "source": "hypothetical"},
      "embedding_tokens_per_search": {"value": 50, "source": "hypothetical"}, "emails": {"value": 7, "source": "observed"}}
  }
}`

func TestRun_RendersThePlanAndTheCostTables(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, run(strings.NewReader(inlineInputs), &out))
	s := out.String()
	assert.Contains(t, s, "| Base requests per day | 2,000,000 | requests/day | derived |")
	assert.Contains(t, s, "| Monthly active participants | 1,000,000 | participants | hypothetical |")
	assert.Contains(t, s, "Peak / measured capacity")
	assert.Contains(t, s, "Cost per activated room")
	assert.Contains(t, s, "hypothetical placeholder", "the price source is printed beside the costs")
	assert.Contains(t, s, "not a forecast")
}

func TestRun_AMissingPriceIsNamedNotZero(t *testing.T) {
	var out bytes.Buffer
	in := strings.Replace(inlineInputs, `"egress_per_gb": 0.05, `, "", 1)
	require.NoError(t, run(strings.NewReader(in), &out))
	assert.Contains(t, out.String(), "missing: egress_per_gb")
	assert.NotContains(t, out.String(), "Cost per activated room |")
}

func TestRun_RejectsUnknownFields(t *testing.T) {
	var out bytes.Buffer
	err := run(strings.NewReader(`{"capacity": {"monthly_actives": {"value": 1}}}`), &out)
	require.Error(t, err, "a misspelled input must not silently become zero")
}

// The committed example inputs parse and render.
func TestExampleInputs_Render(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "..", "docs", "ops", "model-inputs.example.json"))
	require.NoError(t, err)
	defer f.Close()
	var out bytes.Buffer
	require.NoError(t, run(f, &out))
	assert.Contains(t, out.String(), "2,000,000")
}
