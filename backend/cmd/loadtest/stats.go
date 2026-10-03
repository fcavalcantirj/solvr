package main

import (
	"math"
	"regexp"
	"sort"
	"sync"
	"time"
)

// Percentile is the p-th percentile of v with linear interpolation between the closest
// ranks — PostgreSQL's percentile_cont — so the harness and GET /admin/ops/slo agree on
// the same samples. No samples is NaN, never zero.
func Percentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	pos := p * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return s[lo] + (pos-float64(lo))*(s[hi]-s[lo])
}

type classStats struct {
	latencies       []float64 // ms, successful requests only
	byStatus        map[int]int
	count           int
	errors          int
	transportErrors int
}

// Recorder collects per-class results. It is safe for concurrent use.
type Recorder struct {
	mu      sync.Mutex
	byClass map[Class]*classStats
}

// NewRecorder creates an empty recorder.
func NewRecorder() *Recorder { return &Recorder{byClass: map[Class]*classStats{}} }

// Observe records one request: its status (0 when the transport failed), its duration
// and the transport error, if any. A non-2xx answer or a transport error is an error.
func (r *Recorder) Observe(c Class, status int, d time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.byClass[c]
	if s == nil {
		s = &classStats{byStatus: map[int]int{}}
		r.byClass[c] = s
	}
	s.count++
	switch {
	case err != nil:
		s.errors++
		s.transportErrors++
	case status < 200 || status > 299:
		s.errors++
		s.byStatus[status]++
	default:
		s.byStatus[status]++
		s.latencies = append(s.latencies, float64(d)/float64(time.Millisecond))
	}
}

// ClassReport is one class's results over a stage.
type ClassReport struct {
	Class           Class       `json:"class"`
	Count           int         `json:"count"`
	RPS             float64     `json:"rps"`
	P50Ms           float64     `json:"p50_ms"`
	P95Ms           float64     `json:"p95_ms"`
	P99Ms           float64     `json:"p99_ms"`
	MaxMs           float64     `json:"max_ms"`
	Errors          int         `json:"errors"`
	TransportErrors int         `json:"transport_errors"`
	ByStatus        map[int]int `json:"by_status"`
}

// Snapshot reports every class over the elapsed time, in class order.
func (r *Recorder) Snapshot(elapsed time.Duration) []ClassReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ClassReport
	for c, s := range r.byClass {
		rep := ClassReport{Class: c, Count: s.count, Errors: s.errors, TransportErrors: s.transportErrors, ByStatus: map[int]int{}}
		for k, v := range s.byStatus {
			rep.ByStatus[k] = v
		}
		if elapsed > 0 {
			rep.RPS = float64(s.count) / elapsed.Seconds()
		}
		rep.P50Ms = jsonSafe(Percentile(s.latencies, 0.50))
		rep.P95Ms = jsonSafe(Percentile(s.latencies, 0.95))
		rep.P99Ms = jsonSafe(Percentile(s.latencies, 0.99))
		rep.MaxMs = jsonSafe(Percentile(s.latencies, 1))
		out = append(out, rep)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Class < out[j].Class })
	return out
}

// jsonSafe keeps NaN out of JSON: -1 marks "no samples" in the report.
func jsonSafe(v float64) float64 {
	if math.IsNaN(v) {
		return -1
	}
	return v
}

var noncePattern = regexp.MustCompile(`lt-[0-9a-f]{16}`)

// nonceIn returns the harness nonce a frame's data carries, or "".
func nonceIn(data string) string { return noncePattern.FindString(data) }

// DeliveryTracker measures connected-client delivery: from the instant a write was sent
// to the instant each stream subscriber received the frame carrying its nonce. Sender and
// receivers share one clock (one process), so no clock skew enters the figure.
type DeliveryTracker struct {
	mu      sync.Mutex
	sent    map[string]time.Time
	samples []float64
}

// NewDeliveryTracker creates an empty tracker.
func NewDeliveryTracker() *DeliveryTracker { return &DeliveryTracker{sent: map[string]time.Time{}} }

// Sent records when the write carrying nonce was sent.
func (d *DeliveryTracker) Sent(nonce string, at time.Time) {
	d.mu.Lock()
	d.sent[nonce] = at
	d.mu.Unlock()
}

// Received records one subscriber receiving nonce; a nonce this run never sent is ignored.
func (d *DeliveryTracker) Received(nonce string, at time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if sent, ok := d.sent[nonce]; ok {
		d.samples = append(d.samples, float64(at.Sub(sent))/float64(time.Millisecond))
	}
}

// DeliveryReport is connected-client delivery latency over a stage.
type DeliveryReport struct {
	Samples int     `json:"samples"`
	P50Ms   float64 `json:"p50_ms"`
	P95Ms   float64 `json:"p95_ms"`
	P99Ms   float64 `json:"p99_ms"`
}

// Report summarises the samples so far.
func (d *DeliveryTracker) Report() DeliveryReport {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DeliveryReport{
		Samples: len(d.samples),
		P50Ms:   jsonSafe(Percentile(d.samples, 0.50)),
		P95Ms:   jsonSafe(Percentile(d.samples, 0.95)),
		P99Ms:   jsonSafe(Percentile(d.samples, 0.99)),
	}
}

// Reset starts a new stage: samples are dropped, nonces in flight are kept.
func (d *DeliveryTracker) Reset() {
	d.mu.Lock()
	d.samples = nil
	d.mu.Unlock()
}
