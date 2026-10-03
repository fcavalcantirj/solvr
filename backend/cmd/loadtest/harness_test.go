package main

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The load harness's pure parts (spec.json idx 79 step 2). The run itself is evidence, not
// a test: it is executed once against a local API and its report is kept.

func TestPicker_FollowsTheRecordedMixDeterministically(t *testing.T) {
	mix := Mix{ClassOverview: 0.8, ClassPoll: 0.1, ClassWrite: 0.08, ClassSearch: 0.02}
	p, err := NewPicker(mix, 42)
	require.NoError(t, err)
	counts := map[Class]int{}
	const n = 100_000
	for i := 0; i < n; i++ {
		counts[p.Pick()]++
	}
	for class, weight := range mix {
		assert.InDelta(t, weight, float64(counts[class])/n, 0.01, string(class))
	}

	again, _ := NewPicker(mix, 42)
	first, _ := NewPicker(mix, 42)
	for i := 0; i < 100; i++ {
		assert.Equal(t, first.Pick(), again.Pick(), "the same seed replays the same sequence")
	}
}

func TestPicker_RefusesAnEmptyOrNegativeMix(t *testing.T) {
	_, err := NewPicker(Mix{}, 1)
	assert.Error(t, err)
	_, err = NewPicker(Mix{ClassOverview: -1, ClassSearch: 2}, 1)
	assert.Error(t, err)
	_, err = NewPicker(Mix{"unknown": 1}, 1)
	assert.Error(t, err)
}

func TestLoadMix_ReadsTheRecordedMixFile(t *testing.T) {
	m, err := parseMixFile(strings.NewReader(`{"source":"recorded","classes":{"overview_read":0.5,"search":0.5},"streams":7}`))
	require.NoError(t, err)
	assert.Equal(t, 0.5, m.Classes[ClassSearch])
	assert.Equal(t, 7, m.Streams)
	assert.Equal(t, "recorded", m.Source)
}

// Percentiles interpolate like PostgreSQL's percentile_cont, so the harness and
// GET /admin/ops/slo compute the same p95 from the same samples.
func TestPercentile_MatchesPercentileCont(t *testing.T) {
	var v []float64
	for i := 1; i <= 100; i++ {
		v = append(v, float64(i))
	}
	assert.InDelta(t, 95.05, Percentile(v, 0.95), 1e-9)
	assert.InDelta(t, 50.5, Percentile(v, 0.50), 1e-9)
	assert.InDelta(t, 385, Percentile([]float64{400, 100, 300, 200}, 0.95), 1e-9, "unsorted input is sorted first")
	assert.Equal(t, 7.0, Percentile([]float64{7}, 0.99))
	assert.True(t, Percentile(nil, 0.95) != Percentile(nil, 0.95), "no samples is NaN, never zero")
}

func TestRecorder_CountsStatusesErrorsAndLatencies(t *testing.T) {
	r := NewRecorder()
	r.Observe(ClassOverview, 200, 10*time.Millisecond, nil)
	r.Observe(ClassOverview, 200, 30*time.Millisecond, nil)
	r.Observe(ClassOverview, 500, 5*time.Millisecond, nil)
	r.Observe(ClassWrite, 0, time.Second, assert.AnError)
	r.Observe(ClassWrite, 429, time.Millisecond, nil)

	reports := r.Snapshot(2 * time.Second)
	byClass := map[Class]ClassReport{}
	for _, c := range reports {
		byClass[c.Class] = c
	}
	ov := byClass[ClassOverview]
	assert.Equal(t, 3, ov.Count)
	assert.Equal(t, 1, ov.Errors, "a 5xx is an error")
	assert.InDelta(t, 1.5, ov.RPS, 1e-9)
	assert.InDelta(t, 29, ov.P95Ms, 1e-9, "latency percentiles are over successful requests: 10, 30")
	assert.Equal(t, map[int]int{200: 2, 500: 1}, ov.ByStatus)

	wr := byClass[ClassWrite]
	assert.Equal(t, 2, wr.Errors)
	assert.Equal(t, 1, wr.TransportErrors)
	assert.Equal(t, 1, wr.ByStatus[429])
}

func TestReadSSE_ParsesFramesAndSkipsComments(t *testing.T) {
	stream := ": heartbeat\n\nretry: 1000\n\nid: 12\nevent: message\ndata: {\"body\":\"load lt-0123456789abcdef\"}\n\n" +
		"event: presence_join\ndata: {}\n\n"
	var got []sseEvent
	require.NoError(t, readSSE(strings.NewReader(stream), func(e sseEvent) { got = append(got, e) }))
	require.Len(t, got, 2)
	assert.Equal(t, "12", got[0].ID)
	assert.Equal(t, "message", got[0].Event)
	assert.Equal(t, "lt-0123456789abcdef", nonceIn(got[0].Data))
	assert.Equal(t, "presence_join", got[1].Event)
	assert.Empty(t, nonceIn(got[1].Data))
}

func TestDeliveryTracker_MeasuresSendToReceive(t *testing.T) {
	d := NewDeliveryTracker()
	t0 := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	d.Sent("lt-aaaaaaaaaaaaaaaa", t0)
	d.Received("lt-aaaaaaaaaaaaaaaa", t0.Add(150*time.Millisecond))
	d.Received("lt-aaaaaaaaaaaaaaaa", t0.Add(250*time.Millisecond)) // a second subscriber
	d.Received("lt-unknown00000000", t0.Add(time.Second))           // not ours: ignored
	rep := d.Report()
	assert.Equal(t, 2, rep.Samples)
	assert.InDelta(t, 245, rep.P95Ms, 1e-9)
}

func TestParseLoadAvg_DarwinAndLinux(t *testing.T) {
	got, err := parseLoadAvg("{ 3.27 4.31 5.36 }\n")
	require.NoError(t, err)
	assert.Equal(t, [3]float64{3.27, 4.31, 5.36}, got)
	got, err = parseLoadAvg("0.50 0.40 0.30 1/123 456\n")
	require.NoError(t, err)
	assert.Equal(t, [3]float64{0.5, 0.4, 0.3}, got)
	_, err = parseLoadAvg("garbage")
	assert.Error(t, err)
}

func TestParseStages(t *testing.T) {
	stages, err := ParseStages("10:60s,25:1m30s")
	require.NoError(t, err)
	assert.Equal(t, []Stage{{TargetRPS: 10, Duration: time.Minute}, {TargetRPS: 25, Duration: 90 * time.Second}}, stages)
	_, err = ParseStages("10")
	assert.Error(t, err)
	_, err = ParseStages("-5:10s")
	assert.Error(t, err)
}

// An open-loop stage fires at the target rate regardless of how slow responses are; past
// the in-flight cap a request is counted as dropped by the client, never silently skipped.
func TestRunStage_OpenLoopAtTheTargetRateWithAnInFlightCap(t *testing.T) {
	var fired atomic.Int64
	res := runStage(context.Background(), Stage{TargetRPS: 200, Duration: 500 * time.Millisecond}, 1000,
		func(context.Context) { fired.Add(1) })
	assert.InDelta(t, 100, res.Scheduled, 10)
	assert.Equal(t, res.Scheduled, int(fired.Load())+res.Dropped)
	assert.Zero(t, res.Dropped)

	// Five slow requests hold every slot until after the stage's arrivals end.
	block := make(chan struct{})
	time.AfterFunc(400*time.Millisecond, func() { close(block) })
	res = runStage(context.Background(), Stage{TargetRPS: 200, Duration: 300 * time.Millisecond}, 5,
		func(context.Context) { <-block })
	assert.Greater(t, res.Dropped, 0, "past the in-flight cap requests are dropped and counted")
}

func TestKnee_IsTheHighestStageThatMetEveryTarget(t *testing.T) {
	stages := []StageReport{
		{TargetRPS: 10, AchievedRPS: 10, ErrorRate: 0, ReadP95Ms: 20, WriteP95Ms: 40, DeliveryP95Ms: 100},
		{TargetRPS: 50, AchievedRPS: 49, ErrorRate: 0.001, ReadP95Ms: 80, WriteP95Ms: 120, DeliveryP95Ms: 300},
		{TargetRPS: 100, AchievedRPS: 70, ErrorRate: 0, ReadP95Ms: 90, WriteP95Ms: 100, DeliveryP95Ms: 200},
		{TargetRPS: 200, AchievedRPS: 199, ErrorRate: 0, ReadP95Ms: 700, WriteP95Ms: 100, DeliveryP95Ms: 200},
	}
	assert.Equal(t, 50.0, Knee(stages), "100 missed its rate and 200 missed the read target")
	assert.Equal(t, 0.0, Knee(nil))
}

func TestReport_MarkdownIsLabelledLaptop(t *testing.T) {
	rep := Report{Machine: Machine{Label: LaptopLabel, NumCPU: 8}, Stages: []StageReport{{TargetRPS: 10, AchievedRPS: 9.8}}}
	md := rep.Markdown()
	assert.Contains(t, md, "LAPTOP")
	assert.Contains(t, md, "not production capacity")
	assert.Contains(t, md, "| 10 |")
}
