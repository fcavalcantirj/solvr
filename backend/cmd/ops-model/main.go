// Command ops-model prints the capacity planning model and the cost model (spec.json idx 79
// steps 3 and 4) from a JSON inputs file, as markdown tables:
//
//	go run ./cmd/ops-model -inputs ../docs/ops/model-inputs.example.json
//
// Every input carries its source (observed or hypothetical). Every price is required and
// has no default; the example's prices are hypothetical placeholders and the real ones are
// the owner's (UAT). A missing price is named, never read as zero.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/ops"
)

type inputs struct {
	Label       string             `json:"label"`
	PriceSource string             `json:"price_source"`
	Capacity    ops.CapacityInputs `json:"capacity"`
	Cost        struct {
		Prices ops.CostPrices `json:"prices"`
		Usage  ops.CostUsage  `json:"usage"`
	} `json:"cost"`
}

func main() {
	path := flag.String("inputs", "", "JSON inputs file (see docs/ops/model-inputs.example.json)")
	flag.Parse()
	if *path == "" {
		log.Fatal("-inputs is required")
	}
	f, err := os.Open(*path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := run(f, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(r io.Reader, w io.Writer) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var in inputs
	if err := dec.Decode(&in); err != nil {
		return fmt.Errorf("read inputs: %w", err)
	}

	plan := ops.CapacityPlan(in.Capacity)
	fmt.Fprintf(w, "## Capacity plan%s\n\n_%s._\n\n", labelSuffix(in.Label), plan.Note)
	fmt.Fprintln(w, "| Input or result | Value | Unit | Source |")
	fmt.Fprintln(w, "|---|---:|---|---|")
	for _, row := range plan.Rows {
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n", row.Label, num(row.Value), row.Unit, row.Source)
	}

	fmt.Fprintf(w, "\n## Cost model%s\n\n", labelSuffix(in.Label))
	report, err := ops.CostModel(in.Cost.Prices, in.Cost.Usage)
	if err != nil {
		fmt.Fprintf(w, "Not computed: %s.\n", strings.Replace(err.Error(), "every price is a required input; ", "every price is a required input — ", 1))
		return nil
	}
	fmt.Fprintf(w, "Prices: %s. %s.\n\n", orUnstated(in.PriceSource), report.Note)
	fmt.Fprintln(w, "| Component | Per month | Basis |")
	fmt.Fprintln(w, "|---|---:|---|")
	for _, c := range report.Components {
		fmt.Fprintf(w, "| %s | %s | %s |\n", c.Name, money(c.PerMonth), c.Basis)
	}
	fmt.Fprintf(w, "| **total** | **%s** | |\n\n", money(report.TotalPerMonth))
	fmt.Fprintln(w, "| Unit cost | Units | Fully loaded | Marginal | Status |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---|")
	for _, u := range report.Units {
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n", u.Label, num(u.Units), moneyPtr(u.FullyLoaded), moneyPtr(u.Marginal), u.Status)
	}
	return nil
}

func labelSuffix(label string) string {
	if label == "" {
		return ""
	}
	return " — " + label
}

func orUnstated(s string) string {
	if s == "" {
		return "source unstated"
	}
	return s
}

// num prints integers with thousands separators and fractions with up to four places.
func num(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return grouped(int64(v))
	}
	s := fmt.Sprintf("%.4f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	whole, frac, _ := strings.Cut(s, ".")
	var n int64
	fmt.Sscan(whole, &n)
	out := grouped(n)
	if strings.HasPrefix(whole, "-") && n == 0 {
		out = "-0"
	}
	if frac != "" {
		out += "." + frac
	}
	return out
}

func grouped(n int64) string {
	s := fmt.Sprint(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func money(v float64) string {
	if v != 0 && math.Abs(v) < 0.01 {
		return fmt.Sprintf("%.6f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func moneyPtr(v *float64) string {
	if v == nil {
		return "—"
	}
	return money(*v)
}
