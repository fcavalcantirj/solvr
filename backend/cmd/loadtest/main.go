// Command loadtest drives a local Solvr API with the recorded request mix (spec.json idx 79
// step 2): anonymous overview reads, searches, agent polling and timeline writes at a stated
// target rate per stage, while long-lived streams stay connected and measure delivery. It
// samples the machine's load average throughout. Its numbers are LAPTOP numbers — the API,
// PostgreSQL and the harness share one machine — and never production capacity.
//
//	go run ./cmd/loadtest -base-url http://127.0.0.1:18650 -mix ../docs/ops/load-mix.json \
//	    -out-json /tmp/solvr-lane-o/load.json -out-md /tmp/solvr-lane-o/load.md
//
// It only ever talks to the base URL it is given; scripts/ops/run-load-test.sh starts a local
// API on a production-shaped copy for it.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	mrand "math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

func main() {
	var (
		baseURL     = flag.String("base-url", "http://127.0.0.1:18650", "API to drive (a local one)")
		stagesFlag  = flag.String("stages", "10:60s,25:60s,50:60s,100:60s,200:60s,400:60s", "rps:duration,...")
		maxInFlight = flag.Int("max-in-flight", 256, "most requests in flight at once")
		agents      = flag.Int("agents", 40, "agents to register")
		rooms       = flag.Int("rooms", 8, "public rooms the agents work in")
		streams     = flag.Int("streams", -1, "streams to hold open (-1: the mix file's)")
		mixPath     = flag.String("mix", "../docs/ops/load-mix.json", "recorded mix file")
		seed        = flag.Int64("seed", 79, "seed for the class picker")
		baseline    = flag.Duration("baseline", 30*time.Second, "idle load-average sampling before the run")
		maxErrRate  = flag.Float64("stop-error-rate", 0.05, "stop escalating past a stage with more errors than this")
		dataset     = flag.String("dataset", "unstated", "what the API is serving, for the report")
		outJSON     = flag.String("out-json", "", "write the JSON report here")
		outMD       = flag.String("out-md", "", "write the markdown report here")
		deadline    = flag.Duration("deadline", 20*time.Minute, "hard limit for the whole run")
	)
	flag.Parse()

	stages, err := ParseStages(*stagesFlag)
	if err != nil {
		log.Fatal(err)
	}
	mix, err := loadMixFile(*mixPath)
	if err != nil {
		log.Fatal(err)
	}
	if *streams < 0 {
		*streams = mix.Streams
	}
	picker, err := NewPicker(mix.Classes, *seed)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *deadline)
	defer cancel()

	transport := &http.Transport{MaxIdleConns: *maxInFlight + *streams + 16, MaxIdleConnsPerHost: *maxInFlight + *streams + 16,
		MaxConnsPerHost: *maxInFlight + *streams + 16, IdleConnTimeout: 90 * time.Second}
	t := &target{base: strings.TrimRight(*baseURL, "/"), client: &http.Client{Transport: transport, Timeout: 10 * time.Second},
		runID: randomHex(3)}

	rep := Report{StartedAt: time.Now(), Machine: machine(),
		Config: RunConfig{BaseURL: t.base, Stages: *stagesFlag, MaxInFlight: *maxInFlight, Agents: *agents, Rooms: *rooms,
			Streams: *streams, Mix: mix.Classes, MixSource: mix.Source, Seed: *seed, Dataset: *dataset}}

	if err := t.waitHealthy(ctx, time.Minute); err != nil {
		log.Fatal(err)
	}
	log.Printf("baseline: sampling load average for %s", *baseline)
	bs := StartLoadSampler(5 * time.Second)
	time.Sleep(*baseline)
	rep.Baseline = summariseLoad(bs.Stop())

	log.Printf("setup: %d agents, %d rooms", *agents, *rooms)
	if err := t.setup(ctx, *agents, *rooms); err != nil {
		log.Fatal(err)
	}

	tracker := NewDeliveryTracker()
	streamCtx, stopStreams := context.WithCancel(ctx)
	rep.Streams = t.openStreams(streamCtx, *streams, tracker)
	log.Printf("streams: %d opened, %d failed", rep.Streams.Opened, rep.Streams.Failed)

	var pickMu sync.Mutex
	for _, st := range stages {
		rec := NewRecorder()
		tracker.Reset()
		sampler := StartLoadSampler(5 * time.Second)
		log.Printf("stage: %s RPS for %s", rate(st.TargetRPS), st.Duration)
		run := runStage(ctx, st, *maxInFlight, func(ctx context.Context) {
			pickMu.Lock()
			class := picker.Pick()
			pickMu.Unlock()
			t.fire(ctx, class, rec, tracker)
		})
		time.Sleep(3 * time.Second) // let the last writes reach the streams
		sr := summariseStage(st, run, rec.Snapshot(run.Elapsed), tracker.Report(), summariseLoad(sampler.Stop()))
		rep.Stages = append(rep.Stages, sr)
		log.Printf("stage %s: achieved %.1f RPS, errors %.2f%%, read p95 %s, write p95 %s, delivery p95 %s",
			rate(st.TargetRPS), sr.AchievedRPS, 100*sr.ErrorRate, ms(sr.ReadP95Ms), ms(sr.WriteP95Ms), ms(sr.DeliveryP95Ms))
		if sr.ErrorRate > *maxErrRate {
			rep.StoppedBy = fmt.Sprintf("stage %s RPS error rate %.2f%% > %.0f%%", rate(st.TargetRPS), 100*sr.ErrorRate, 100**maxErrRate)
			break
		}
		if ctx.Err() != nil {
			rep.StoppedBy = "deadline"
			break
		}
	}
	stopStreams()
	rep.Streams.EndedEarly = int(t.streamsEnded.Load())
	rep.KneeRPS = Knee(rep.Stages)
	rep.FinishedAt = time.Now()
	rep.Config.WriteShare = mix.Classes[ClassWrite]

	if *outJSON != "" {
		raw, _ := json.MarshalIndent(rep, "", "  ")
		if err := os.WriteFile(*outJSON, raw, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	md := rep.Markdown()
	if *outMD != "" {
		if err := os.WriteFile(*outMD, []byte(md), 0o644); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Print(md)
}

var overviewPaths = []func(t *target) string{
	func(*target) string { return "/v1/overview" },
	func(*target) string { return "/v1/posts?per_page=20" },
	func(t *target) string { return "/v1/posts/" + t.postIDs[mrand.IntN(len(t.postIDs))] },
	func(*target) string { return "/v1/rooms" },
	func(t *target) string { return "/v1/rooms/" + t.slugs[mrand.IntN(len(t.slugs))] + "/entries?limit=50" },
}

var pollPaths = []func(a agent) string{
	func(agent) string { return "/v1/notifications" },
	func(agent) string { return "/v1/me" },
	func(a agent) string { return "/v1/rooms/" + a.slug + "/entries?limit=20" },
}

// fire sends one request of the class and records it.
func (t *target) fire(ctx context.Context, class Class, rec *Recorder, tracker *DeliveryTracker) {
	var status int
	var err error
	start := time.Now()
	switch class {
	case ClassOverview:
		path := overviewPaths[mrand.IntN(len(overviewPaths))](t)
		status, _, err = t.do(ctx, http.MethodGet, path, "", t.anonIPs[mrand.IntN(len(t.anonIPs))], nil)
	case ClassSearch:
		q := url.QueryEscape(t.terms[mrand.IntN(len(t.terms))])
		status, _, err = t.do(ctx, http.MethodGet, "/v1/search?q="+q+"&per_page=10", "", t.anonIPs[mrand.IntN(len(t.anonIPs))], nil)
	case ClassPoll:
		a := t.agents[mrand.IntN(len(t.agents))]
		status, _, err = t.do(ctx, http.MethodGet, pollPaths[mrand.IntN(len(pollPaths))](a), a.key, a.ip, nil)
	case ClassWrite:
		a := t.agents[mrand.IntN(len(t.agents))]
		nonce := "lt-" + randomHex(8)
		start = time.Now()
		tracker.Sent(nonce, start)
		status, _, err = t.do(ctx, http.MethodPost, "/v1/rooms/"+a.slug+"/entries", a.key, a.ip,
			map[string]string{"kind": "message", "body": "load test " + nonce, "client_entry_id": uuid.NewString()})
	}
	if ctx.Err() != nil {
		return // the run ended mid-request: not a result
	}
	rec.Observe(class, status, time.Since(start), err)
}

// openStreams connects n anonymous subscribers across the rooms; each reports the frames
// carrying a harness nonce to the delivery tracker until ctx ends.
func (t *target) openStreams(ctx context.Context, n int, tracker *DeliveryTracker) StreamReport {
	rep := StreamReport{Requested: n}
	streamClient := &http.Client{Transport: t.client.Transport} // no timeout: streams are long-lived
	var opened, failed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.base+"/v1/rooms/"+t.slugs[i%len(t.slugs)]+"/stream", nil)
			if err != nil {
				failed.Add(1)
				return
			}
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("CF-Connecting-IP", clientIP(79, i))
			resp, err := streamClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				failed.Add(1)
				if resp != nil {
					resp.Body.Close()
				}
				return
			}
			opened.Add(1)
			go func() {
				defer resp.Body.Close()
				_ = readSSE(resp.Body, func(e sseEvent) {
					if nonce := nonceIn(e.Data); nonce != "" {
						tracker.Received(nonce, time.Now())
					}
				})
				if ctx.Err() == nil {
					t.streamsEnded.Add(1)
				}
			}()
		}(i)
	}
	wg.Wait()
	rep.Opened, rep.Failed = int(opened.Load()), int(failed.Load())
	return rep
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func machine() Machine {
	m := Machine{Label: LaptopLabel, OS: runtime.GOOS + "/" + runtime.GOARCH, NumCPU: runtime.NumCPU(),
		Note: "The API, PostgreSQL (Docker) and this harness share the machine, alongside other development work."}
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		m.MemBytes, _ = strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	}
	return m
}
