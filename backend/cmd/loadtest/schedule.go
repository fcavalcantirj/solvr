package main

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Stage is one constant-arrival-rate step of the run.
type Stage struct {
	TargetRPS float64
	Duration  time.Duration
}

// ParseStages reads "rps:duration,rps:duration", e.g. "10:60s,25:60s".
func ParseStages(s string) ([]Stage, error) {
	var out []Stage
	for _, part := range strings.Split(s, ",") {
		rps, dur, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			return nil, fmt.Errorf("stage %q: want rps:duration", part)
		}
		r, err := strconv.ParseFloat(rps, 64)
		if err != nil || r <= 0 {
			return nil, fmt.Errorf("stage %q: rps must be a positive number", part)
		}
		d, err := time.ParseDuration(dur)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("stage %q: bad duration", part)
		}
		out = append(out, Stage{TargetRPS: r, Duration: d})
	}
	return out, nil
}

// StageRun is what the scheduler did in one stage.
type StageRun struct {
	Scheduled int
	Dropped   int
	Elapsed   time.Duration
}

// runStage fires requests open-loop at the target rate for the stage's duration: arrivals
// do not wait for responses, as real clients do not. At most maxInFlight run at once; an
// arrival past the cap is dropped by the client and counted, never silently skipped. It
// waits for every fired request before returning.
func runStage(ctx context.Context, st Stage, maxInFlight int, fire func(context.Context)) StageRun {
	var res StageRun
	sem := make(chan struct{}, maxInFlight)
	var wg sync.WaitGroup
	start := time.Now()
	interval := time.Duration(float64(time.Second) / st.TargetRPS)
	for i := 0; ; i++ {
		next := start.Add(time.Duration(i) * interval)
		if next.Sub(start) >= st.Duration {
			break
		}
		if d := time.Until(next); d > 0 {
			select {
			case <-ctx.Done():
				wg.Wait()
				res.Elapsed = time.Since(start)
				return res
			case <-time.After(d):
			}
		}
		res.Scheduled++
		select {
		case sem <- struct{}{}:
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				fire(ctx)
			}()
		default:
			res.Dropped++
		}
	}
	res.Elapsed = time.Since(start)
	wg.Wait()
	return res
}

// LoadSample is the machine's load average at one instant.
type LoadSample struct {
	At      time.Time `json:"at"`
	One     float64   `json:"one"`
	Five    float64   `json:"five"`
	Fifteen float64   `json:"fifteen"`
}

// parseLoadAvg reads the three load averages from `sysctl -n vm.loadavg` ("{ 3.27 4.31
// 5.36 }") or /proc/loadavg ("0.50 0.40 0.30 1/123 456").
func parseLoadAvg(s string) ([3]float64, error) {
	var out [3]float64
	fields := strings.Fields(strings.NewReplacer("{", " ", "}", " ").Replace(s))
	if len(fields) < 3 {
		return out, fmt.Errorf("load average: %q", s)
	}
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return out, fmt.Errorf("load average: %q", s)
		}
		out[i] = v
	}
	return out, nil
}

func readLoadAvg() ([3]float64, error) {
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return [3]float64{}, err
	}
	return parseLoadAvg(string(out))
}

// LoadSampler samples the load average on an interval until stopped.
type LoadSampler struct {
	mu      sync.Mutex
	samples []LoadSample
	stop    chan struct{}
	done    chan struct{}
}

// StartLoadSampler samples now and then every interval.
func StartLoadSampler(every time.Duration) *LoadSampler {
	s := &LoadSampler{stop: make(chan struct{}), done: make(chan struct{})}
	take := func() {
		if v, err := readLoadAvg(); err == nil {
			s.mu.Lock()
			s.samples = append(s.samples, LoadSample{At: time.Now(), One: v[0], Five: v[1], Fifteen: v[2]})
			s.mu.Unlock()
		}
	}
	take()
	go func() {
		defer close(s.done)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				take()
				return
			case <-t.C:
				take()
			}
		}
	}()
	return s
}

// Stop ends sampling and returns every sample.
func (s *LoadSampler) Stop() []LoadSample {
	close(s.stop)
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]LoadSample(nil), s.samples...)
}

// LoadSummary is the 1-minute load average's range over a span.
type LoadSummary struct {
	Samples int     `json:"samples"`
	Min     float64 `json:"min_1m"`
	Mean    float64 `json:"mean_1m"`
	Max     float64 `json:"max_1m"`
}

func summariseLoad(samples []LoadSample) LoadSummary {
	ls := LoadSummary{Samples: len(samples)}
	for i, s := range samples {
		if i == 0 || s.One < ls.Min {
			ls.Min = s.One
		}
		if s.One > ls.Max {
			ls.Max = s.One
		}
		ls.Mean += s.One
	}
	if len(samples) > 0 {
		ls.Mean /= float64(len(samples))
	}
	return ls
}
