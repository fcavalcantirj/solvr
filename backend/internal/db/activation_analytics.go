package db

import (
	"context"
	"fmt"
	"time"
)

// Measuring activation.
//
// A room is ACTIVATED when at least two distinct authenticated agent identities
// have each posted a real message in it — the connection funnel records that as
// the deduplicated first_two_way_exchange milestone (see funnel_events.go), once
// per room whether the room holds two agents or eight. This file reads those
// server-recorded steps back into the numbers an operator needs: how many rooms
// were created, how many activated, how long the milestones took, and how the
// funnel converted, split by where the attempt came from.
//
// Everything here is INTERNAL operator analytics. It aggregates across public
// AND private rooms, but a row it reads carries no message body, task text or
// room title — only an opaque room_id, a non-secret flow_id, timestamps and the
// step's shape — so an aggregate exposes no content. The handler that serves it
// sits behind the operator gate.
//
// Two definitions are reported side by side and never conflated:
//   - PROSPECTIVE activation: the strict first_two_way_exchange milestone, which
//     counts one authenticated identity once even when it posts under two display
//     names, so a single agent talking to itself does not "activate" a room.
//   - HISTORICAL multi-author participation: the looser, older count of rooms
//     with two or more distinct message display names. It is labeled as history,
//     not activation.

// ActivationConversion is one funnel conversion: how many of a denominator went
// on to the numerator. Denominator and Rate are pointers because a denominator
// is not always known — an agent that created a room straight from a pasted
// prompt left no browser "started" event to divide by — and a rate over an
// unknown or zero denominator is reported as absent rather than as zero.
type ActivationConversion struct {
	Denominator *int     `json:"denominator"`
	Numerator   int      `json:"numerator"`
	Rate        *float64 `json:"rate"`
}

// newConversion builds a conversion with a known denominator, leaving the rate
// absent when the denominator is zero (no valid ratio) rather than dividing.
func newConversion(denominator, numerator int) ActivationConversion {
	c := ActivationConversion{Denominator: &denominator, Numerator: numerator}
	if denominator > 0 {
		rate := float64(numerator) / float64(denominator)
		c.Rate = &rate
	}
	return c
}

// numeratorOnly builds a conversion whose denominator is unknown: the count is
// real, but there is nothing valid to divide it by, so no rate is claimed.
func numeratorOnly(numerator int) ActivationConversion {
	return ActivationConversion{Numerator: numerator}
}

// ActivationByOrigin splits flow-to-room conversion by where the connection
// attempt originated. Website attempts have a browser "started" event to divide
// by; direct-API and unknown-origin rooms do not, so they report a count only.
//
// Website counts FLOWS on both sides: the distinct flows a browser started in the
// window, and how many of those flows produced at least one room in it. A sentence
// pasted into two agents makes two rooms and still one converted flow, so the rate
// never passes 1. DirectAPI and Unknown count ROOMS: rooms whose flow code no website
// visit started, and rooms that carried no code. The rooms of website flows are what is
// left of RoomsCreated.
type ActivationByOrigin struct {
	Website   ActivationConversion `json:"website"`
	DirectAPI ActivationConversion `json:"direct_api"`
	Unknown   ActivationConversion `json:"unknown"`
}

// WebsiteFlowSteps follows the website-started flows of the window (the flows whose
// browser connection_started is in it) through the steps that tie a visit to a room:
// how many distinct flows reached each step inside the window. SkillFetched counts only
// an agent's fetch (entry_surface agent_fetch): a person who opened the skill link in a
// browser is not an agent that read it, and neither is a link preview or a crawler
// (bot_fetch). Started is flow_to_room.website's denominator and RoomCreated its numerator.
type WebsiteFlowSteps struct {
	Started      int `json:"started"`
	PromptCopied int `json:"prompt_copied"`
	SkillFetched int `json:"skill_fetched"`
	RoomCreated  int `json:"room_created"`
}

// ActivationDurationStats is a median/p90 distribution over a set of per-room
// durations in milliseconds. Median and P90 are pointers so an empty set reads
// as "no measurement" instead of a misleading zero.
type ActivationDurationStats struct {
	Count  int      `json:"count"`
	Median *float64 `json:"median_ms"`
	P90    *float64 `json:"p90_ms"`
}

// ActivationParticipants measures how many agents joined rooms and how many
// rooms grew past the activating pair, kept separate from activation: a room
// with eight agents is one room and one activation, not a count of pairs.
type ActivationParticipants struct {
	RoomsWithParticipants int `json:"rooms_with_participants"`
	MaxParticipants       int `json:"max_participants"`
	RoomsWithLaterJoins   int `json:"rooms_with_later_joins"`
}

// ActivationReport is the full activation measurement for one window.
type ActivationReport struct {
	WindowFrom time.Time `json:"window_start"`
	WindowTo   time.Time `json:"window_end"`
	GeneratedAt time.Time `json:"generated_at"`

	ActivationDefinition string `json:"activation_definition"`

	RoomsCreated     int                  `json:"rooms_created"`
	ActivatedRooms   int                  `json:"activated_rooms"`
	RoomToActivation ActivationConversion `json:"room_to_activation"`
	FlowToRoom       ActivationByOrigin   `json:"flow_to_room"`
	WebsiteFlowSteps WebsiteFlowSteps     `json:"website_flow_steps"`

	TimeToSecondAgentMS   ActivationDurationStats `json:"time_to_second_agent_ms"`
	TimeToFirstExchangeMS ActivationDurationStats `json:"time_to_first_exchange_ms"`

	Participants ActivationParticipants `json:"participants"`

	HistoricalMultiAuthorRooms int    `json:"historical_multi_author_rooms"`
	HistoricalNote             string `json:"historical_note"`
}

// activationDefinitionText and historicalNoteText are the documented meaning of
// the two numbers, carried in the response so a reader never has to guess which
// definition a figure uses.
const (
	activationDefinitionText = "A room is activated when at least two distinct authenticated agent " +
		"identities have each posted a non-empty, non-system message in it after joining. Presence, " +
		"system messages and bootstrap-readiness messages are excluded, and the milestone is counted once " +
		"per room regardless of how many agents join."
	historicalNoteText = "Historical multi-author participation counts rooms with two or more distinct " +
		"message display names, the looser definition that predates prospective activation instrumentation. " +
		"It is not the same as, and is never added to, the strict prospective activation count."
)

// ActivationAnalyticsRepository reads activation measurements from the funnel.
type ActivationAnalyticsRepository struct {
	pool *Pool
}

// NewActivationAnalyticsRepository creates the activation analytics reader.
func NewActivationAnalyticsRepository(pool *Pool) *ActivationAnalyticsRepository {
	return &ActivationAnalyticsRepository{pool: pool}
}

// Measure computes the activation report over rooms whose room_created step
// occurred in [from, to). The range is closed-open so adjacent windows do not
// double-count a room at the boundary.
func (r *ActivationAnalyticsRepository) Measure(ctx context.Context, from, to time.Time) (ActivationReport, error) {
	rep := ActivationReport{
		WindowFrom:           from,
		WindowTo:             to,
		GeneratedAt:          time.Now().UTC(),
		ActivationDefinition: activationDefinitionText,
		HistoricalNote:       historicalNoteText,
	}

	if err := r.measureCounts(ctx, from, to, &rep); err != nil {
		return ActivationReport{}, err
	}
	if err := r.measureDurations(ctx, from, to, &rep); err != nil {
		return ActivationReport{}, err
	}
	if err := r.measureParticipants(ctx, from, to, &rep); err != nil {
		return ActivationReport{}, err
	}
	if err := r.measureHistorical(ctx, &rep); err != nil {
		return ActivationReport{}, err
	}
	return rep, nil
}

// measureCounts fills rooms created, activated rooms, room->activation, the
// flow->room conversion split by origin and the website flows' steps, all from the
// funnel steps of the window.
func (r *ActivationAnalyticsRepository) measureCounts(ctx context.Context, from, to time.Time, rep *ActivationReport) error {
	var roomsCreated, activated, directRooms, unknownRooms int
	var steps WebsiteFlowSteps
	err := r.pool.QueryRow(ctx, `
		WITH created AS (
			SELECT DISTINCT ON (room_id) room_id, flow_id
			  FROM funnel_events
			 WHERE event_name = 'room_created' AND room_id IS NOT NULL
			   AND occurred_at >= $1 AND occurred_at < $2
		),
		started AS (
			SELECT DISTINCT flow_id
			  FROM funnel_events
			 WHERE event_name = 'connection_started' AND source_channel = 'browser'
			   AND flow_id IS NOT NULL
			   AND occurred_at >= $1 AND occurred_at < $2
		),
		reached AS (
			SELECT DISTINCT f.event_name, f.flow_id
			  FROM funnel_events f
			 WHERE f.flow_id IN (SELECT flow_id FROM started)
			   AND f.occurred_at >= $1 AND f.occurred_at < $2
			   AND (f.event_name = 'starter_prompt_copied'
			        OR (f.event_name = 'skill_fetched' AND f.entry_surface = 'agent_fetch'))
		)
		SELECT
			(SELECT COUNT(*) FROM created),
			(SELECT COUNT(DISTINCT f.room_id)
			   FROM funnel_events f
			  WHERE f.event_name = 'first_two_way_exchange'
			    AND f.room_id IN (SELECT room_id FROM created)),
			(SELECT COUNT(*) FROM started),
			(SELECT COUNT(DISTINCT c.flow_id) FROM created c
			  WHERE c.flow_id IN (SELECT flow_id FROM started)),
			(SELECT COUNT(*) FROM created c
			  WHERE c.flow_id IS NOT NULL
			    AND c.flow_id NOT IN (SELECT flow_id FROM started)),
			(SELECT COUNT(*) FROM created c WHERE c.flow_id IS NULL),
			(SELECT COUNT(*) FROM reached WHERE event_name = 'starter_prompt_copied'),
			(SELECT COUNT(*) FROM reached WHERE event_name = 'skill_fetched')
	`, from, to).Scan(&roomsCreated, &activated, &steps.Started, &steps.RoomCreated, &directRooms, &unknownRooms,
		&steps.PromptCopied, &steps.SkillFetched)
	if err != nil {
		LogQueryError(ctx, "ActivationCounts", "funnel_events", err)
		return fmt.Errorf("measure activation counts: %w", err)
	}

	rep.RoomsCreated = roomsCreated
	rep.ActivatedRooms = activated
	rep.RoomToActivation = newConversion(roomsCreated, activated)
	rep.FlowToRoom = ActivationByOrigin{
		// Flows on both sides: steps.RoomCreated is the website-started flows that
		// produced at least one room, never the number of rooms they produced.
		Website:   newConversion(steps.Started, steps.RoomCreated),
		DirectAPI: numeratorOnly(directRooms),
		Unknown:   numeratorOnly(unknownRooms),
	}
	rep.WebsiteFlowSteps = steps
	return nil
}

// measureDurations fills the two milestone-latency distributions. Each is the
// gap from a room's creation to a later milestone, in milliseconds, summarized
// with the continuous median and p90.
func (r *ActivationAnalyticsRepository) measureDurations(ctx context.Context, from, to time.Time, rep *ActivationReport) error {
	second, err := r.durationStats(ctx, from, to, "participant_joined", 2)
	if err != nil {
		return err
	}
	exchange, err := r.durationStats(ctx, from, to, "first_two_way_exchange", 0)
	if err != nil {
		return err
	}
	rep.TimeToSecondAgentMS = second
	rep.TimeToFirstExchangeMS = exchange
	return nil
}

// durationStats measures the millisecond gap from each room's creation to its
// milestone step. When ordinal > 0 the milestone is a participant_joined at that
// ordinal (the second agent to join); otherwise it is the named once-per-room
// step. percentile_cont returns NULL for an empty set, which scans into the nil
// pointers, so an empty window reads as "no measurement".
func (r *ActivationAnalyticsRepository) durationStats(ctx context.Context, from, to time.Time, event string, ordinal int) (ActivationDurationStats, error) {
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
			    ON m.room_id = c.room_id
			   AND m.event_name = $3
			   AND ($4 = 0 OR m.ordinal = $4)
			 WHERE c.event_name = 'room_created' AND c.room_id IS NOT NULL
			   AND c.occurred_at >= $1 AND c.occurred_at < $2
			   AND m.occurred_at >= c.occurred_at
		  ) t
	`, from, to, event, ordinal).Scan(&stats.Median, &stats.P90, &stats.Count)
	if err != nil {
		LogQueryError(ctx, "ActivationDurations", "funnel_events", err)
		return ActivationDurationStats{}, fmt.Errorf("measure activation durations (%s): %w", event, err)
	}
	return stats, nil
}

// measureParticipants counts, over rooms created in the window, how many drew
// participants, the largest participant count reached, and how many grew past
// the activating pair to a third or later agent.
func (r *ActivationAnalyticsRepository) measureParticipants(ctx context.Context, from, to time.Time, rep *ActivationReport) error {
	var p ActivationParticipants
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COALESCE(MAX(max_ord), 0),
			COUNT(*) FILTER (WHERE max_ord >= 3)
		  FROM (
			SELECT f.room_id, MAX(f.ordinal) AS max_ord
			  FROM funnel_events f
			  JOIN funnel_events c
			    ON c.room_id = f.room_id
			   AND c.event_name = 'room_created'
			   AND c.occurred_at >= $1 AND c.occurred_at < $2
			 WHERE f.event_name = 'participant_joined' AND f.ordinal IS NOT NULL
			 GROUP BY f.room_id
		  ) t
	`, from, to).Scan(&p.RoomsWithParticipants, &p.MaxParticipants, &p.RoomsWithLaterJoins)
	if err != nil {
		LogQueryError(ctx, "ActivationParticipants", "funnel_events", err)
		return fmt.Errorf("measure activation participants: %w", err)
	}
	rep.Participants = p
	return nil
}

// measureHistorical counts the looser, older definition — rooms with two or more
// distinct message display names — across the whole message history. It is
// reported beside prospective activation, labeled, never merged into it.
func (r *ActivationAnalyticsRepository) measureHistorical(ctx context.Context, rep *ActivationReport) error {
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
		LogQueryError(ctx, "ActivationHistorical", "messages", err)
		return fmt.Errorf("measure historical multi-author rooms: %w", err)
	}
	rep.HistoricalMultiAuthorRooms = count
	return nil
}
