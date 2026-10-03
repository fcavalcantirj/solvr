package db

import (
	"context"
	"fmt"
	"time"
)

// Measuring the share loop (idx 88 steps 5-6).
//
// Everything is read from the connection funnel (funnel_events): the browser share
// steps (share_visit, share_link_copied), the Try-this arrivals (connection_started
// with a source) and the server steps of rooms created from a public source, which
// carry that source from room_created through activation (see funnel_events.go). It
// is INTERNAL operator analytics: opaque ids and pseudonymous refs only, no titles,
// bodies or tasks. Visits are visits, never people; humans are counted only from
// identified human rows, never guessed from anonymous ones or from agents.

// ShareVisitSplit counts share visits by the actor the API classified.
type ShareVisitSplit struct {
	Total     int `json:"total"`
	Human     int `json:"human"`
	Agent     int `json:"agent"`
	Anonymous int `json:"anonymous"`
}

// ShareReturns is one return horizon: of the identities whose horizon has fully
// elapsed (Eligible), how many came back for other work within it (Returned).
type ShareReturns struct {
	Eligible int `json:"eligible"`
	Returned int `json:"returned"`
}

// ShareK is the referral coefficient, an EXPERIMENT metric: k = referred visits per
// activated origin × referred-visit activation rate. It is not a growth guarantee.
type ShareK struct {
	Value            *float64 `json:"value"`
	ExperimentMetric bool     `json:"experiment_metric"`
	Formula          string   `json:"formula"`
}

// ShareAttributionReport is the share loop over one window.
type ShareAttributionReport struct {
	ShareVisits              ShareVisitSplit `json:"share_visits"`
	Invitations              int             `json:"invitations"`
	ReferredVisits           int             `json:"referred_visits"`
	AttributedFlows          int             `json:"attributed_flows"`
	AttributedRoomsCreated   int             `json:"attributed_rooms_created"`
	AttributedRoomsActivated int             `json:"attributed_rooms_activated"`

	Origins                          int                  `json:"origins"`
	ActivatedOrigins                 int                  `json:"activated_origins"`
	InvitationsPerActivatedOrigin    *float64             `json:"invitations_per_activated_origin"`
	ReferredVisitsPerActivatedOrigin *float64             `json:"referred_visits_per_activated_origin"`
	InviteToActivation               ActivationConversion `json:"invite_to_activation"`
	ReferredVisitActivation          ActivationConversion `json:"referred_visit_activation"`
	K                                ShareK               `json:"k"`

	NewAgentActivations        int `json:"new_agent_activations"`
	NewHumanActivations        int `json:"new_human_activations"`
	UnresolvedHumanActivations int `json:"unresolved_human_activations"`

	AgentReturns7d  ShareReturns `json:"agent_returns_7d"`
	AgentReturns28d ShareReturns `json:"agent_returns_28d"`
	HumanReturns7d  ShareReturns `json:"human_returns_7d"`
	HumanReturns28d ShareReturns `json:"human_returns_28d"`

	Definitions map[string]string `json:"definitions"`
	Caveats     []string          `json:"caveats"`
}

// ShareAttributionRepository measures the share loop.
type ShareAttributionRepository struct {
	pool *Pool
}

// NewShareAttributionRepository creates the share-attribution reader.
func NewShareAttributionRepository(pool *Pool) *ShareAttributionRepository {
	return &ShareAttributionRepository{pool: pool}
}

type shareOrigin struct{ kind, id string }

// attributedRoom is a room created from a public source inside the window.
type attributedRoom struct {
	room, flow  string
	activatedAt *time.Time
}

// Measure reports the share loop for steps that happened in [from, to). Activation and
// returns are judged as of now, and a return horizon counts only once it has elapsed.
func (r *ShareAttributionRepository) Measure(ctx context.Context, from, to, now time.Time) (ShareAttributionReport, error) {
	rep := ShareAttributionReport{
		K: ShareK{ExperimentMetric: true,
			Formula: "referred_visits_per_activated_origin × referred_visit_activation.rate"},
		Definitions: shareDefinitions(),
		Caveats:     shareCaveats(),
	}

	referredBy, invitedBy, err := r.readBrowserSteps(ctx, from, to, &rep)
	if err != nil {
		return rep, err
	}
	activatedOrigins, err := r.activatedOrigins(ctx, referredBy, invitedBy, now)
	if err != nil {
		return rep, err
	}
	rep.Origins = len(unionOrigins(referredBy, invitedBy))
	rep.ActivatedOrigins = len(activatedOrigins)
	var referredFromActivated, invitedFromActivated int
	for o := range activatedOrigins {
		referredFromActivated += referredBy[o]
		invitedFromActivated += invitedBy[o]
	}
	rep.ReferredVisitsPerActivatedOrigin = perOrigin(referredFromActivated, rep.ActivatedOrigins)
	rep.InvitationsPerActivatedOrigin = perOrigin(invitedFromActivated, rep.ActivatedOrigins)

	rooms, err := r.attributedRooms(ctx, from, to, now)
	if err != nil {
		return rep, err
	}
	rep.AttributedRoomsCreated = len(rooms)
	var activated []attributedRoom
	for _, room := range rooms {
		if room.activatedAt != nil {
			activated = append(activated, room)
		}
	}
	rep.AttributedRoomsActivated = len(activated)
	rep.InviteToActivation = newConversion(rep.Invitations, rep.AttributedRoomsActivated)
	rep.ReferredVisitActivation = newConversion(rep.ReferredVisits, rep.AttributedRoomsActivated)
	if rep.ReferredVisitsPerActivatedOrigin != nil && rep.ReferredVisitActivation.Rate != nil {
		k := *rep.ReferredVisitsPerActivatedOrigin * *rep.ReferredVisitActivation.Rate
		rep.K.Value = &k
	}

	if err := r.measureActivationsAndReturns(ctx, activated, now, &rep); err != nil {
		return rep, err
	}
	return rep, nil
}

// readBrowserSteps counts the window's sourced browser steps and returns, per origin,
// its referred visits (share visits + Try-this arrivals) and its invitations (copies).
func (r *ShareAttributionRepository) readBrowserSteps(ctx context.Context, from, to time.Time, rep *ShareAttributionReport) (map[shareOrigin]int, map[shareOrigin]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT event_name, actor_type, source_kind, source_id::text, COALESCE(flow_id, '')
		  FROM funnel_events
		 WHERE occurred_at >= $1 AND occurred_at < $2 AND source_id IS NOT NULL
		   AND event_name IN ('share_visit', 'share_link_copied', 'connection_started')`, from, to)
	if err != nil {
		LogQueryError(ctx, "ShareAttribution.browser", "funnel_events", err)
		return nil, nil, fmt.Errorf("share attribution browser steps: %w", err)
	}
	defer rows.Close()

	referred, invited := map[shareOrigin]int{}, map[shareOrigin]int{}
	flows := map[string]bool{}
	for rows.Next() {
		var event, actor, kind, id, flow string
		if err := rows.Scan(&event, &actor, &kind, &id, &flow); err != nil {
			return nil, nil, fmt.Errorf("scan share step: %w", err)
		}
		o := shareOrigin{kind, id}
		switch event {
		case "share_visit":
			rep.ShareVisits.Total++
			switch actor {
			case "human":
				rep.ShareVisits.Human++
			case "agent":
				rep.ShareVisits.Agent++
			default:
				rep.ShareVisits.Anonymous++
			}
			referred[o]++
		case "connection_started":
			referred[o]++
			if flow != "" {
				flows[flow] = true
			}
		case "share_link_copied":
			rep.Invitations++
			invited[o]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	for _, n := range referred {
		rep.ReferredVisits += n
	}
	rep.AttributedFlows = len(flows)
	return referred, invited, nil
}

// activatedOrigins keeps the origins that stand for a useful collaboration: a room that
// reached its own two-way exchange by now, or a post (only a published public post
// resolves as a source).
func (r *ShareAttributionRepository) activatedOrigins(ctx context.Context, referred, invited map[shareOrigin]int, now time.Time) (map[shareOrigin]bool, error) {
	all := unionOrigins(referred, invited)
	out := map[shareOrigin]bool{}
	var roomIDs []string
	for o := range all {
		if o.kind == "post" {
			out[o] = true
		} else {
			roomIDs = append(roomIDs, o.id)
		}
	}
	if len(roomIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT room_id::text FROM funnel_events
		 WHERE event_name = 'first_two_way_exchange' AND room_id = ANY($1::uuid[]) AND occurred_at <= $2`,
		roomIDs, now)
	if err != nil {
		LogQueryError(ctx, "ShareAttribution.origins", "funnel_events", err)
		return nil, fmt.Errorf("share attribution origins: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan activated origin: %w", err)
		}
		out[shareOrigin{"room", id}] = true
	}
	return out, rows.Err()
}

// attributedRooms lists rooms created from a public source in the window, with their
// activation instant when they reached a two-way exchange by now.
func (r *ShareAttributionRepository) attributedRooms(ctx context.Context, from, to, now time.Time) ([]attributedRoom, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.room_id::text, COALESCE(c.flow_id, ''), x.occurred_at
		  FROM funnel_events c
		  LEFT JOIN funnel_events x
		    ON x.room_id = c.room_id AND x.event_name = 'first_two_way_exchange' AND x.occurred_at <= $3
		 WHERE c.event_name = 'room_created' AND c.source_id IS NOT NULL
		   AND c.occurred_at >= $1 AND c.occurred_at < $2`, from, to, now)
	if err != nil {
		LogQueryError(ctx, "ShareAttribution.rooms", "funnel_events", err)
		return nil, fmt.Errorf("share attribution rooms: %w", err)
	}
	defer rows.Close()
	var out []attributedRoom
	for rows.Next() {
		var a attributedRoom
		if err := rows.Scan(&a.room, &a.flow, &a.activatedAt); err != nil {
			return nil, fmt.Errorf("scan attributed room: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func unionOrigins(a, b map[shareOrigin]int) map[shareOrigin]bool {
	out := map[shareOrigin]bool{}
	for o := range a {
		out[o] = true
	}
	for o := range b {
		out[o] = true
	}
	return out
}

func perOrigin(n, origins int) *float64 {
	if origins == 0 {
		return nil
	}
	v := float64(n) / float64(origins)
	return &v
}
