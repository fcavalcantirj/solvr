package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
)

// Class is a request class of the recorded mix.
type Class string

// The request classes spec.json idx 79 step 2 names. Long-lived streams are not a class
// of the request mix: they are held open for the whole run (Mix.Streams).
const (
	ClassOverview Class = "overview_read"  // anonymous overview, post and room reads
	ClassSearch   Class = "search"         // GET /v1/search
	ClassPoll     Class = "agent_poll"     // an agent's authenticated reads while it waits
	ClassWrite    Class = "timeline_write" // POST /v1/rooms/{slug}/entries
)

var knownClasses = map[Class]bool{ClassOverview: true, ClassSearch: true, ClassPoll: true, ClassWrite: true}

// Mix is the share of requests each class takes.
type Mix map[Class]float64

// MixFile is docs/ops/load-mix.json: the recorded mix, where it came from, and the number
// of streams held open.
type MixFile struct {
	Source     string             `json:"source"`
	Derivation []string           `json:"derivation"`
	Classes    Mix                `json:"classes"`
	Streams    int                `json:"streams"`
	StreamNote string             `json:"stream_note"`
	Recorded   map[string]float64 `json:"recorded"`
}

func loadMixFile(path string) (MixFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return MixFile{}, err
	}
	defer f.Close()
	return parseMixFile(f)
}

func parseMixFile(r io.Reader) (MixFile, error) {
	var m MixFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, fmt.Errorf("read mix: %w", err)
	}
	return m, nil
}

// Picker draws classes in proportion to a mix from a seeded source, so a run replays.
type Picker struct {
	classes []Class
	cum     []float64
	rng     *rand.Rand
}

// NewPicker validates the mix and builds a picker over it.
func NewPicker(m Mix, seed int64) (*Picker, error) {
	var classes []Class
	total := 0.0
	for c, w := range m {
		if !knownClasses[c] {
			return nil, fmt.Errorf("unknown class %q", c)
		}
		if w < 0 {
			return nil, fmt.Errorf("class %q has a negative weight", c)
		}
		if w > 0 {
			classes = append(classes, c)
			total += w
		}
	}
	if total == 0 {
		return nil, fmt.Errorf("the mix is empty")
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] }) // map order is random
	p := &Picker{classes: classes, rng: rand.New(rand.NewSource(seed))}
	acc := 0.0
	for _, c := range classes {
		acc += m[c] / total
		p.cum = append(p.cum, acc)
	}
	return p, nil
}

// Pick draws one class. It is not safe for concurrent use; the scheduler calls it from
// one goroutine.
func (p *Picker) Pick() Class {
	x := p.rng.Float64()
	for i, c := range p.cum {
		if x < c {
			return p.classes[i]
		}
	}
	return p.classes[len(p.classes)-1]
}
