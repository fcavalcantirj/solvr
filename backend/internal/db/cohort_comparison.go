package db

import (
	"context"
	"fmt"
	"time"
)

// Comparing the redesigned experience over consistent post-launch cohorts.
//
// A single launch timestamp anchors every window, and the same definitions and
// exclusions apply to each, so a seven-day cohort and a twenty-eight-day cohort
// can be read side by side without a moving all-time baseline drifting under
// them. Everything here reads the connection funnel (see funnel_events.go) plus
// the rooms and posts a room links to; it is INTERNAL operator analytics behind
// the operator gate, aggregating public AND private rooms while exposing no
// message body, task text or room title — only opaque ids, timestamps and step
// shapes.
//
// Two things are reported but never conflated: how many rooms were CREATED and
// how many rooms reached a two-way exchange (COLLABORATION). A rise in creation
// with a flat or falling conversion is a different story from a rise in
// successful collaboration, and the report keeps both legible.

// CohortVisibilitySplit splits the rooms created in a window by room visibility.
// Unknown counts a created room whose funnel step has no matching rooms row (a
// room pruned or never resolvable) — reported honestly, never guessed.
type CohortVisibilitySplit struct {
	PublicRooms  int `json:"public_rooms"`
	PrivateRooms int `json:"private_rooms"`
	Unknown      int `json:"unknown_visibility"`
}

// CohortCreatorSplit splits the rooms created in a window by whether the creating
// identity is seen for the first time inside the window (new) or had earlier
// funnel activity (returning). Unknown counts a creator with no pseudonymous ref.
type CohortCreatorSplit struct {
	NewCreator       int `json:"new_creator"`
	ReturningCreator int `json:"returning_creator"`
	Unknown          int `json:"unknown_creator"`
}

// CohortWindow is one post-launch window, measured from the launch instant with
// definitions identical to every other window.
type CohortWindow struct {
	Days        int       `json:"days"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	// Complete is true only when the whole window has elapsed (now >= WindowEnd).
	// A partial window still reports its numbers, labeled incomplete, rather than
	// being omitted or zero-filled.
	Complete bool `json:"complete"`

	RoomsCreated     int                  `json:"rooms_created"`
	ActivatedRooms   int                  `json:"activated_rooms"`
	RoomToActivation ActivationConversion `json:"room_to_activation"`
	// SecondAgentConnectionFailures counts rooms where a second agent actually
	// joined (a participant at ordinal >= 2) but the two-way exchange never
	// happened — the connection was attempted and the collaboration failed, which
	// is distinct from a room where no second agent ever arrived.
	SecondAgentConnectionFailures int                     `json:"second_agent_connection_failures"`
	TimeToFirstExchangeMS         ActivationDurationStats `json:"time_to_first_exchange_ms"`
	// RoomsWithReturn counts rooms with a later agent session after their two-way
	// exchange — a participant joining or a room view after activation.
	RoomsWithReturn        int `json:"rooms_with_return"`
	RoomToPostPublications int `json:"room_to_post_publications"`

	Visibility CohortVisibilitySplit `json:"visibility"`
	Creators   CohortCreatorSplit    `json:"creators"`
}

// CohortExternalSource marks whether an external analytics source is connected.
// GA and GSC are not wired as data sources, so they report unavailable rather
// than contributing zeros that would read as "measured and empty".
type CohortExternalSource struct {
	Available bool   `json:"available"`
	Note      string `json:"note"`
}

// CohortHistoricalContext carries the all-time, message-based multi-author room
// count as CONTEXT beside the cohort windows. It uses the older, looser
// definition and is never a comparable activation baseline.
type CohortHistoricalContext struct {
	AllTimeMultiAuthorRooms int    `json:"all_time_multi_author_rooms"`
	Note                    string `json:"note"`
}

// CohortExternalSources groups the external sources the report cannot measure.
type CohortExternalSources struct {
	GoogleAnalytics CohortExternalSource `json:"google_analytics"`
	SearchConsole   CohortExternalSource `json:"search_console"`
}

// CohortComparisonReport compares consistent post-launch cohort windows.
type CohortComparisonReport struct {
	Launch      time.Time `json:"launch"`
	GeneratedAt time.Time `json:"generated_at"`

	ActivationDefinition string `json:"activation_definition"`
	ExclusionsNote       string `json:"exclusions_note"`

	Window7d  CohortWindow `json:"window_7d"`
	Window28d CohortWindow `json:"window_28d"`

	Historical      CohortHistoricalContext `json:"historical_context"`
	ExternalSources CohortExternalSources   `json:"external_sources"`
}

// Documented meanings carried in the response so a reader never guesses which
// definition a figure uses or why a source is empty.
const (
	cohortExclusionsNote = "Rooms are split by visibility (from the room record) and by whether the " +
		"creating identity is new or returning (from earlier funnel activity). Owner/internal test traffic " +
		"and automated probes are not separately identifiable from the stored funnel — no origin flag is " +
		"recorded — so they are neither excluded by guess nor attributed to a human; a created room with no " +
		"resolvable record is reported under unknown rather than assigned a visibility."
	cohortHistoricalNote = "All-time multi-author rooms use the older, looser definition (two or more distinct " +
		"message display names) and are provided as historical context. They are not a directly comparable " +
		"room-activation baseline and are never added to a cohort window."
	gaUnavailableNote  = "Google Analytics is not connected as a data source; browser acquisition and traffic " +
		"figures are unavailable and are reported as absent, not zero."
	gscUnavailableNote = "Google Search Console is not connected as a data source; search-impression and " +
		"click figures are unavailable and are reported as absent, not zero."
)

// CohortComparisonRepository reads post-launch cohort comparisons from the funnel.
type CohortComparisonRepository struct {
	pool *Pool
}

// NewCohortComparisonRepository creates the cohort comparison reader.
func NewCohortComparisonRepository(pool *Pool) *CohortComparisonRepository {
	return &CohortComparisonRepository{pool: pool}
}

// Compare builds the report for the 7-day and 28-day windows that start at
// launch, judged complete against now. Both windows share one launch, one
// timezone (UTC) and one set of definitions.
func (r *CohortComparisonRepository) Compare(ctx context.Context, launch, now time.Time) (CohortComparisonReport, error) {
	launch = launch.UTC()
	now = now.UTC()
	rep := CohortComparisonReport{
		Launch:               launch,
		GeneratedAt:          now,
		ActivationDefinition: activationDefinitionText,
		ExclusionsNote:       cohortExclusionsNote,
		ExternalSources: CohortExternalSources{
			GoogleAnalytics: CohortExternalSource{Available: false, Note: gaUnavailableNote},
			SearchConsole:   CohortExternalSource{Available: false, Note: gscUnavailableNote},
		},
		Historical: CohortHistoricalContext{Note: cohortHistoricalNote},
	}

	w7, err := r.measureWindow(ctx, launch, 7, now)
	if err != nil {
		return CohortComparisonReport{}, err
	}
	w28, err := r.measureWindow(ctx, launch, 28, now)
	if err != nil {
		return CohortComparisonReport{}, err
	}
	rep.Window7d = w7
	rep.Window28d = w28

	if err := r.measureHistorical(ctx, &rep); err != nil {
		return CohortComparisonReport{}, err
	}
	return rep, nil
}

// measureWindow computes one [launch, launch+days) window with identical
// definitions to every other window.
func (r *CohortComparisonRepository) measureWindow(ctx context.Context, launch time.Time, days int, now time.Time) (CohortWindow, error) {
	from := launch
	to := launch.Add(time.Duration(days) * 24 * time.Hour)
	w := CohortWindow{
		Days:        days,
		WindowStart: from,
		WindowEnd:   to,
		Complete:    !now.Before(to),
	}

	if err := r.measureCounts(ctx, from, to, &w); err != nil {
		return CohortWindow{}, err
	}
	stats, err := r.firstExchangeDuration(ctx, from, to)
	if err != nil {
		return CohortWindow{}, err
	}
	w.TimeToFirstExchangeMS = stats
	if err := r.measureReturnsAndPosts(ctx, from, to, &w); err != nil {
		return CohortWindow{}, err
	}
	return w, nil
}

// measureCounts fills rooms created, activation, second-agent failures, and the
// visibility and creator splits, all over rooms whose room_created step falls in
// [from, to).
func (r *CohortComparisonRepository) measureCounts(ctx context.Context, from, to time.Time, w *CohortWindow) error {
	var created, activated, failures, public, private, unknownVis, newCreator, returningCreator, unknownCreator int
	err := r.pool.QueryRow(ctx, `
		WITH created AS (
			SELECT DISTINCT ON (room_id) room_id, actor_ref
			  FROM funnel_events
			 WHERE event_name = 'room_created' AND room_id IS NOT NULL
			   AND occurred_at >= $1 AND occurred_at < $2
			 ORDER BY room_id, occurred_at
		),
		activated AS (
			SELECT DISTINCT room_id FROM funnel_events
			 WHERE event_name = 'first_two_way_exchange' AND room_id IS NOT NULL
		),
		attempted AS (
			SELECT DISTINCT room_id FROM funnel_events
			 WHERE event_name = 'participant_joined' AND room_id IS NOT NULL
			   AND ordinal IS NOT NULL AND ordinal >= 2
		),
		firstseen AS (
			SELECT actor_ref, MIN(occurred_at) AS first_at
			  FROM funnel_events WHERE actor_ref IS NOT NULL GROUP BY actor_ref
		)
		SELECT
			(SELECT COUNT(*) FROM created),
			(SELECT COUNT(*) FROM created c WHERE c.room_id IN (SELECT room_id FROM activated)),
			(SELECT COUNT(*) FROM created c
			  WHERE c.room_id IN (SELECT room_id FROM attempted)
			    AND c.room_id NOT IN (SELECT room_id FROM activated)),
			(SELECT COUNT(*) FROM created c JOIN rooms r ON r.id = c.room_id WHERE r.is_private = false),
			(SELECT COUNT(*) FROM created c JOIN rooms r ON r.id = c.room_id WHERE r.is_private = true),
			(SELECT COUNT(*) FROM created c WHERE NOT EXISTS (SELECT 1 FROM rooms r WHERE r.id = c.room_id)),
			(SELECT COUNT(*) FROM created c JOIN firstseen fs ON fs.actor_ref = c.actor_ref
			  WHERE c.actor_ref IS NOT NULL AND fs.first_at >= $1),
			(SELECT COUNT(*) FROM created c JOIN firstseen fs ON fs.actor_ref = c.actor_ref
			  WHERE c.actor_ref IS NOT NULL AND fs.first_at < $1),
			(SELECT COUNT(*) FROM created c WHERE c.actor_ref IS NULL)
	`, from, to).Scan(&created, &activated, &failures, &public, &private, &unknownVis,
		&newCreator, &returningCreator, &unknownCreator)
	if err != nil {
		LogQueryError(ctx, "CohortCounts", "funnel_events", err)
		return fmt.Errorf("measure cohort counts: %w", err)
	}

	w.RoomsCreated = created
	w.ActivatedRooms = activated
	w.RoomToActivation = newConversion(created, activated)
	w.SecondAgentConnectionFailures = failures
	w.Visibility = CohortVisibilitySplit{PublicRooms: public, PrivateRooms: private, Unknown: unknownVis}
	w.Creators = CohortCreatorSplit{NewCreator: newCreator, ReturningCreator: returningCreator, Unknown: unknownCreator}
	return nil
}

// firstExchangeDuration measures the millisecond gap from a room's creation to
// its two-way exchange, summarized with the continuous median and p90.
// percentile_cont returns NULL for an empty set, scanning into nil pointers, so
// an empty window reads as "no measurement" rather than zero.
func (r *CohortComparisonRepository) firstExchangeDuration(ctx context.Context, from, to time.Time) (ActivationDurationStats, error) {
	var stats ActivationDurationStats
	err := r.pool.QueryRow(ctx, `
		SELECT
			percentile_cont(0.5) WITHIN GROUP (ORDER BY d),
			percentile_cont(0.9) WITHIN GROUP (ORDER BY d),
			COUNT(*)
		  FROM (
			SELECT EXTRACT(EPOCH FROM (m.occurred_at - c.occurred_at)) * 1000 AS d
			  FROM funnel_events c
			  JOIN funnel_events m
			    ON m.room_id = c.room_id AND m.event_name = 'first_two_way_exchange'
			 WHERE c.event_name = 'room_created' AND c.room_id IS NOT NULL
			   AND c.occurred_at >= $1 AND c.occurred_at < $2
			   AND m.occurred_at >= c.occurred_at
		  ) t
	`, from, to).Scan(&stats.Median, &stats.P90, &stats.Count)
	if err != nil {
		LogQueryError(ctx, "CohortDurations", "funnel_events", err)
		return ActivationDurationStats{}, fmt.Errorf("measure cohort first-exchange durations: %w", err)
	}
	return stats, nil
}

// measureReturnsAndPosts fills how many created rooms saw a later agent session
// after their exchange, and how many produced a linked Post.
func (r *CohortComparisonRepository) measureReturnsAndPosts(ctx context.Context, from, to time.Time, w *CohortWindow) error {
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT c.room_id)
		  FROM funnel_events c
		  JOIN funnel_events ex
		    ON ex.room_id = c.room_id AND ex.event_name = 'first_two_way_exchange'
		  JOIN funnel_events later
		    ON later.room_id = c.room_id
		   AND later.event_name IN ('participant_joined', 'room_viewed')
		   AND later.occurred_at > ex.occurred_at
		 WHERE c.event_name = 'room_created' AND c.room_id IS NOT NULL
		   AND c.occurred_at >= $1 AND c.occurred_at < $2
	`, from, to).Scan(&w.RoomsWithReturn)
	if err != nil {
		LogQueryError(ctx, "CohortReturns", "funnel_events", err)
		return fmt.Errorf("measure cohort returns: %w", err)
	}

	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM posts p
		 WHERE p.source_room_id IS NOT NULL AND p.deleted_at IS NULL
		   AND p.source_room_id IN (
			SELECT room_id FROM funnel_events
			 WHERE event_name = 'room_created' AND room_id IS NOT NULL
			   AND occurred_at >= $1 AND occurred_at < $2)
	`, from, to).Scan(&w.RoomToPostPublications)
	if err != nil {
		LogQueryError(ctx, "CohortPosts", "posts", err)
		return fmt.Errorf("measure cohort room-to-post: %w", err)
	}
	return nil
}

// measureHistorical counts the looser, older definition — rooms with two or more
// distinct message display names — across all message history, reported beside
// the cohorts as context, never merged into a window.
func (r *CohortComparisonRepository) measureHistorical(ctx context.Context, rep *CohortComparisonReport) error {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT room_id
			  FROM messages
			 WHERE author_type = 'agent' AND deleted_at IS NULL
			 GROUP BY room_id
			HAVING COUNT(DISTINCT agent_name) >= 2
		) q
	`).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "CohortHistorical", "messages", err)
		return fmt.Errorf("measure cohort historical context: %w", err)
	}
	rep.Historical.AllTimeMultiAuthorRooms = count
	return nil
}
