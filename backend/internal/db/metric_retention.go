package db

import (
	"time"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// How long raw events are kept, and which metrics read them (spec.json idx 77 step 4).
//
// Every usage figure Solvr reports is computed when it is read, from raw rows. No table stores a
// time-bucketed aggregate: a chart's buckets (searchSeries, apiCallSeries, messageSeries,
// ServiceCheckRepository.GetDailyAggregates) are counts grouped from those rows for one read and
// are never written back, so the raw eligible events stay the only record and every bucket can be
// recomputed from them. Each distinct count (unique queries, distinct operations, rooms with
// conversation, two-way exchanges, unique participants) is one COUNT(DISTINCT ...) over the whole
// window. It is never a sum of per-bucket distinct counts, which counts an identity once for each
// bucket it appears in.
//
// RawEventSources states, for each table those metrics read, how long its rows are kept and the
// longest window each metric reads. A source's retention must cover every reader's window, and
// metric_retention_test.go holds the list to that, to the windows the readers offer, to the code
// that deletes rows, and to a clean installation's schema. Nothing prunes these tables by age
// today. Pruning one deletes history a metric reads, so it is an owner decision: set Retention and
// PrunedBy here in the same change that schedules it.
//
// The one table pruned by age, idempotency_keys (IdempotencyRetention, CleanupJob), is not listed:
// no metric reads it.

// ParticipantWindow is the rolling window of the monthly-active-participant counter; it reads
// that window and the one before it, for returning identities.
const ParticipantWindow = time.Duration(growth.ParticipantWindowDays) * 24 * time.Hour

// KeptIndefinitely is the Retention of a table that nothing prunes by age.
const KeptIndefinitely time.Duration = 0

// AllHistory is the Window of a reader that may read any recorded row: an all-time figure, or a
// window its caller anchors anywhere in the past.
const AllHistory time.Duration = 0

// MetricReader is one metric computed from a raw event table.
type MetricReader struct {
	// Reader is the code that reads the rows, unique within its table.
	Reader string
	// Metric is what the reader reports.
	Metric string
	// Window is the longest span of rows the reader reads, or AllHistory.
	Window time.Duration
}

// RawEventSource is one table of raw events and the metrics read from it.
type RawEventSource struct {
	Table      string
	TimeColumn string
	// Retention is how long a row is kept, or KeptIndefinitely.
	Retention time.Duration
	// PrunedBy names the job that deletes rows by age. Empty when nothing does.
	PrunedBy string
	// RemovedWith names the parent whose deletion takes rows with it, apart from age.
	RemovedWith string
	Readers     []MetricReader
}

// longestRoomStatsWindow is the longest window the homepage and activation selectors offer.
func longestRoomStatsWindow() time.Duration {
	var longest time.Duration
	for _, w := range RoomStatsWindows {
		if w.Duration > longest {
			longest = w.Duration
		}
	}
	return longest
}

// RawEventSources is every raw event table a metric reads.
var RawEventSources = []RawEventSource{
	{
		// SearchAnalyticsRepository.DeleteOlderThan exists, and nothing calls it.
		Table: "search_queries", TimeColumn: "searched_at", Retention: KeptIndefinitely,
		Readers: []MetricReader{
			{"HomepageRepository.GetSearchPulse",
				"homepage search totals, chart and published terms", longestRoomStatsWindow()},
			{"DataAnalyticsRepository",
				"GET /v1/data trending, breakdown and categories (1h, 24h or 7d)", 7 * 24 * time.Hour},
			{"SearchAnalyticsRepository.GetSummary",
				"GET /v1/stats/search over 7 days, and the operator summary over any days the caller asks", AllHistory},
			{"SearchAnalyticsRepository.GetTrending",
				"operator trending and zero-result terms over any days the caller asks", AllHistory},
			{"ParticipantActivityRepository.Measure",
				"operator monthly active participants: searches by identity, anonymous and monitoring searches", 2 * ParticipantWindow},
		},
	},
	{
		Table: "api_request_events", TimeColumn: "occurred_at", Retention: KeptIndefinitely,
		Readers: []MetricReader{
			{"HomepageRepository.GetAPIUsagePulse",
				"homepage API calls, distinct operations and chart", longestRoomStatsWindow()},
			{"HomepageRepository.GetAPIUsagePulse/InstrumentedSince",
				"the earliest recorded call, stated as when measurement began", AllHistory},
			{"ParticipantActivityRepository.Measure",
				"operator participant report traffic: API requests by actor type and operation kind", 2 * ParticipantWindow},
		},
	},
	{
		Table: "funnel_events", TimeColumn: "occurred_at", Retention: KeptIndefinitely,
		Readers: []MetricReader{
			{"ActivationAnalyticsRepository.Measure",
				"operator activation report for rooms created in the last 24h, 7d or 30d", longestRoomStatsWindow()},
			{"CohortComparisonRepository.Compare",
				"post-launch 7- and 28-day cohorts; a creator is new by its first step ever", AllHistory},
			{"FunnelEventRepository.RecordParticipantJoined",
				"a joiner's ordinal and the room's flow id, read as the step is written", AllHistory},
			{"ParticipantActivityRepository.Measure",
				"operator monthly active participants: room creators, anonymous flows and activations", 2 * ParticipantWindow},
			{"GrowthStageRepository.Measure",
				"operator stage gates: weekly activated rooms, owners, workflows, 24h conversion, creator returns (an owner's first room ever)", AllHistory},
		},
	},
	{
		Table: "post_views", TimeColumn: "viewed_at", Retention: KeptIndefinitely,
		RemovedWith: "its post (ON DELETE CASCADE)",
		Readers: []MetricReader{
			{"posts.view_count", "a post's view count, kept by the post_views_count_* triggers", AllHistory},
			{"CanonicalPlatformBriefingRepository.GetTrendingNow", "briefing trending, views in the last 7 days", 7 * 24 * time.Hour},
			{"ParticipantActivityRepository.Measure",
				"operator monthly active participants: readers by identity and anonymous post views", 2 * ParticipantWindow},
		},
	},
	{
		// The messages and room_events views read these rows. A room is deleted only by
		// PresenceReaperJob, within a minute of its expires_at, and no API-created room has one.
		Table: "room_entries", TimeColumn: "created_at", Retention: KeptIndefinitely,
		RemovedWith: "its room (ON DELETE CASCADE)",
		Readers: []MetricReader{
			{"HomepageRepository.GetRoomPulse",
				"rooms with conversation, messages, two-way exchanges and chart", longestRoomStatsWindow()},
			{"HomepageRepository.GetRoomPulse/ActivationInstrumentedSince",
				"the first activation ever, stated as when measurement began", AllHistory},
			{"ActivationAnalyticsRepository.Measure", "the historical multi-author room count", AllHistory},
			{"CohortComparisonRepository.Compare", "the historical multi-author room count", AllHistory},
			{"RoomRepository", "a room's unique participant count", AllHistory},
			{"ParticipantActivityRepository.Measure",
				"operator monthly active participants: room message and event authors, active rooms", 2 * ParticipantWindow},
		},
	},
	{
		// ServiceCheckRepository.DeleteOlderThan exists, and nothing calls it.
		Table: "service_checks", TimeColumn: "checked_at", Retention: KeptIndefinitely,
		Readers: []MetricReader{
			{"StatusHandler.GetStatus",
				"GET /v1/status uptime, average response time and daily history over 30 days", 30 * 24 * time.Hour},
			{"GrowthStageRepository.Measure/Reliability",
				"operator stage 3 reliability gate: operational share of the core services over 30 days", ParticipantWindow},
		},
	},
}
