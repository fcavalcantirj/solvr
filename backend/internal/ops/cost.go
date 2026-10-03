package ops

import (
	"fmt"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// CostPrices are the monthly prices the cost model needs (spec.json idx 79 step 4). Every
// one is a required input with no default: real prices are the owner's (UAT), and a price
// left out is named, never read as zero.
type CostPrices struct {
	ComputePerMonth           *float64 `json:"compute_per_month"`
	DatabasePerMonth          *float64 `json:"database_per_month"`
	StoragePerGBMonth         *float64 `json:"storage_per_gb_month"`
	EgressPerGB               *float64 `json:"egress_per_gb"`
	ObservabilityPerMonth     *float64 `json:"observability_per_month"`
	ObservabilityPerGB        *float64 `json:"observability_per_gb"`
	EmbeddingPerMillionTokens *float64 `json:"embedding_per_million_tokens"`
	EmailPer1000              *float64 `json:"email_per_1000"`
}

// UnitCostComputed marks a unit cost computed from a non-zero unit count. A unit cost has
// no target, so it is never "met"; with zero units it is growth.StatusNotYetMeasurable.
const UnitCostComputed = "computed"

// CostUsage is one month of usage, each figure with its source.
type CostUsage struct {
	ActivatedRooms           growth.Input `json:"activated_rooms"`
	Entries                  growth.Input `json:"entries"`
	RetainedGB               growth.Input `json:"retained_gb"`
	Searches                 growth.Input `json:"searches"`
	ReturningParticipants    growth.Input `json:"returning_participants"`
	EgressGB                 growth.Input `json:"egress_gb"`
	ObservabilityGB          growth.Input `json:"observability_gb"`
	EmbeddingTokensPerSearch growth.Input `json:"embedding_tokens_per_search"`
	Emails                   growth.Input `json:"emails"`
}

// CostComponent is one line of the monthly bill.
type CostComponent struct {
	Name     string  `json:"name"`
	PerMonth float64 `json:"per_month"`
	Basis    string  `json:"basis"`
}

// UnitCost is one cost per unit: fully loaded (the whole month divided by the units) and,
// where a price is directly attributable to one more unit, marginal.
type UnitCost struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Units       float64  `json:"units"`
	FullyLoaded *float64 `json:"fully_loaded"`
	Marginal    *float64 `json:"marginal,omitempty"`
	Status      string   `json:"status"`
}

// CostReport is the cost model's output.
type CostReport struct {
	TotalPerMonth float64         `json:"total_per_month"`
	Components    []CostComponent `json:"components"`
	Units         []UnitCost      `json:"units"`
	Note          string          `json:"note"`
}

func (p CostPrices) missing() []string {
	var out []string
	for _, f := range []struct {
		name string
		v    *float64
	}{
		{"compute_per_month", p.ComputePerMonth}, {"database_per_month", p.DatabasePerMonth},
		{"storage_per_gb_month", p.StoragePerGBMonth}, {"egress_per_gb", p.EgressPerGB},
		{"observability_per_month", p.ObservabilityPerMonth}, {"observability_per_gb", p.ObservabilityPerGB},
		{"embedding_per_million_tokens", p.EmbeddingPerMillionTokens}, {"email_per_1000", p.EmailPer1000},
	} {
		if f.v == nil {
			out = append(out, f.name)
		}
	}
	return out
}

// CostModel computes the monthly bill and the five unit costs from prices and usage.
func CostModel(p CostPrices, u CostUsage) (CostReport, error) {
	if missing := p.missing(); len(missing) > 0 {
		return CostReport{}, fmt.Errorf("every price is a required input; missing: %s", strings.Join(missing, ", "))
	}
	embeddingTokens := u.Searches.Value * u.EmbeddingTokensPerSearch.Value
	components := []CostComponent{
		{"compute", *p.ComputePerMonth, "per month"},
		{"database", *p.DatabasePerMonth, "per month"},
		{"storage", *p.StoragePerGBMonth * u.RetainedGB.Value, "retained GB x price per GB-month"},
		{"egress", *p.EgressPerGB * u.EgressGB.Value, "egress GB x price per GB"},
		{"observability", *p.ObservabilityPerMonth + *p.ObservabilityPerGB*u.ObservabilityGB.Value, "per month + ingested GB x price per GB"},
		{"embeddings", *p.EmbeddingPerMillionTokens * embeddingTokens / 1e6, "searches x tokens per search x price per 1M tokens"},
		{"email", *p.EmailPer1000 * u.Emails.Value / 1000, "emails x price per 1,000"},
	}
	r := CostReport{Components: components,
		Note: "fully loaded = the whole month's bill divided by the units; marginal = the price of one more unit where it is directly attributable"}
	for _, c := range components {
		r.TotalPerMonth += c.PerMonth
	}

	unit := func(key, label string, units, scale float64, marginal *float64) UnitCost {
		uc := UnitCost{Key: key, Label: label, Units: units, Marginal: marginal, Status: growth.StatusNotYetMeasurable}
		if units > 0 {
			v := r.TotalPerMonth / units * scale
			uc.FullyLoaded = &v
			uc.Status = UnitCostComputed
		}
		return uc
	}
	storageMarginal := *p.StoragePerGBMonth
	searchMarginal := *p.EmbeddingPerMillionTokens * u.EmbeddingTokensPerSearch.Value / 1e6
	r.Units = []UnitCost{
		unit("per_activated_room", "Cost per activated room", u.ActivatedRooms.Value, 1, nil),
		unit("per_1000_entries", "Cost per 1,000 entries", u.Entries.Value, 1000, nil),
		unit("per_retained_gb", "Cost per retained GB", u.RetainedGB.Value, 1, &storageMarginal),
		unit("per_search", "Cost per search", u.Searches.Value, 1, &searchMarginal),
		unit("per_returning_participant", "Cost per returning participant", u.ReturningParticipants.Value, 1, nil),
	}
	return r, nil
}
