package ops

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// spec.json idx 79 step 4: cost per activated room, per 1,000 entries, per retained GB, per
// search and per returning participant, egress and observability included. Every price is
// an input with no default; real prices are the owner's (UAT).

func price(v float64) *float64 { return &v }

func fullPrices() CostPrices {
	return CostPrices{
		ComputePerMonth:           price(50),
		DatabasePerMonth:          price(30),
		StoragePerGBMonth:         price(0.10),
		EgressPerGB:               price(0.05),
		ObservabilityPerMonth:     price(10),
		ObservabilityPerGB:        price(0.50),
		EmbeddingPerMillionTokens: price(0.18),
		EmailPer1000:              price(1),
	}
}

func usage() CostUsage {
	return CostUsage{
		ActivatedRooms:           obs(100),
		Entries:                  obs(50_000),
		RetainedGB:               obs(20),
		Searches:                 obs(10_000),
		ReturningParticipants:    obs(400),
		EgressGB:                 obs(100),
		ObservabilityGB:          obs(10),
		EmbeddingTokensPerSearch: hyp(50),
		Emails:                   obs(2_000),
	}
}

func TestCostModel_TotalsEveryComponentIncludingEgressAndObservability(t *testing.T) {
	report, err := CostModel(fullPrices(), usage())
	require.NoError(t, err)

	// compute 50 + database 30 + storage 20×0.10=2 + egress 100×0.05=5 + observability 10 + 10×0.50=5
	// + embeddings 10,000×50/1e6×0.18=0.09 + email 2×1=2  = 104.09
	assert.InDelta(t, 104.09, report.TotalPerMonth, 1e-9)
	byName := map[string]float64{}
	for _, c := range report.Components {
		byName[c.Name] = c.PerMonth
	}
	assert.InDelta(t, 5, byName["egress"], 1e-9)
	assert.InDelta(t, 15, byName["observability"], 1e-9)
	assert.InDelta(t, 0.09, byName["embeddings"], 1e-9)
}

func TestCostModel_UnitCostsAreFullyLoadedWithMarginalWhereAttributable(t *testing.T) {
	report, err := CostModel(fullPrices(), usage())
	require.NoError(t, err)
	unit := map[string]UnitCost{}
	for _, u := range report.Units {
		unit[u.Key] = u
	}
	require.Len(t, unit, 5)

	assertCost := func(key string, want float64) {
		t.Helper()
		require.NotNil(t, unit[key].FullyLoaded, key)
		assert.InDelta(t, want, *unit[key].FullyLoaded, 1e-9, key)
		assert.Equal(t, UnitCostComputed, unit[key].Status, key)
	}
	assertCost("per_activated_room", 104.09/100)
	assertCost("per_1000_entries", 104.09/50_000*1000)
	assertCost("per_retained_gb", 104.09/20)
	assertCost("per_search", 104.09/10_000)
	assertCost("per_returning_participant", 104.09/400)

	require.NotNil(t, unit["per_retained_gb"].Marginal)
	assert.InDelta(t, 0.10, *unit["per_retained_gb"].Marginal, 1e-12, "storing one more GB costs the storage price")
	require.NotNil(t, unit["per_search"].Marginal)
	assert.InDelta(t, 50.0/1e6*0.18, *unit["per_search"].Marginal, 1e-15, "one more search costs its embedding tokens")
}

func TestCostModel_ZeroUnitsAreNotYetMeasurableNeverInfinite(t *testing.T) {
	u := usage()
	u.ActivatedRooms = obs(0)
	report, err := CostModel(fullPrices(), u)
	require.NoError(t, err)
	for _, unit := range report.Units {
		if unit.Key == "per_activated_room" {
			assert.Nil(t, unit.FullyLoaded)
			assert.Equal(t, growth.StatusNotYetMeasurable, unit.Status)
		}
	}
}

// No price has a default: a missing one is named, never assumed to be zero.
func TestCostModel_EveryPriceIsARequiredInput(t *testing.T) {
	p := fullPrices()
	p.EgressPerGB = nil
	p.ObservabilityPerGB = nil
	_, err := CostModel(p, usage())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "egress_per_gb")
	assert.Contains(t, err.Error(), "observability_per_gb")

	_, err = CostModel(CostPrices{}, usage())
	require.Error(t, err)
	for _, name := range []string{"compute_per_month", "database_per_month", "storage_per_gb_month",
		"egress_per_gb", "observability_per_month", "observability_per_gb", "embedding_per_million_tokens", "email_per_1000"} {
		assert.Contains(t, err.Error(), name)
	}
}
