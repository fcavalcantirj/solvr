package db

import (
	"context"
	"fmt"
	"time"
)

// Who the share loop activated, and whether they came back.
//
//   - A NEW AGENT is an agent identity whose first-ever participant_joined is in an
//     attributed room that reached a two-way exchange. A second agent of the same person
//     is still one more agent identity — never one more human.
//   - A NEW HUMAN is an identified human actor_ref on the attributed room's own flow or
//     room rows whose first-ever funnel row is in that flow or room. A room whose rows
//     name no identified human is UNRESOLVED: anonymous people are never guessed.
//   - A RETURN is later work elsewhere within the horizon after the room's activation: an
//     agent joining another room; a human acting on another flow or room (a signed-in
//     room view counts). A horizon is eligible only once it has fully elapsed.

const (
	shareHorizon7d  = 7 * 24 * time.Hour
	shareHorizon28d = 28 * 24 * time.Hour
)

// activatedIdentity is a new agent or human, anchored at its room's activation.
type activatedIdentity struct {
	ref, room, flow string
	at              time.Time
}

func (r *ShareAttributionRepository) measureActivationsAndReturns(ctx context.Context, activated []attributedRoom, now time.Time, rep *ShareAttributionReport) error {
	if len(activated) == 0 {
		return nil
	}
	roomIDs := make([]string, 0, len(activated))
	flows := make([]string, 0, len(activated))
	for _, a := range activated {
		roomIDs = append(roomIDs, a.room)
		if a.flow != "" {
			flows = append(flows, a.flow)
		}
	}

	agents, err := r.newAgents(ctx, activated, roomIDs)
	if err != nil {
		return err
	}
	humans, unresolved, err := r.newHumans(ctx, activated, roomIDs, flows)
	if err != nil {
		return err
	}
	rep.NewAgentActivations = len(agents)
	rep.NewHumanActivations = len(humans)
	rep.UnresolvedHumanActivations = unresolved

	later, err := r.laterActivity(ctx, append(agents, humans...), now)
	if err != nil {
		return err
	}
	rep.AgentReturns7d = countReturns(agents, later, shareHorizon7d, now, true)
	rep.AgentReturns28d = countReturns(agents, later, shareHorizon28d, now, true)
	rep.HumanReturns7d = countReturns(humans, later, shareHorizon7d, now, false)
	rep.HumanReturns28d = countReturns(humans, later, shareHorizon28d, now, false)
	return nil
}

// newAgents returns the agents of the activated rooms whose first-ever join is there.
func (r *ShareAttributionRepository) newAgents(ctx context.Context, activated []attributedRoom, roomIDs []string) ([]activatedIdentity, error) {
	rows, err := r.pool.Query(ctx, `
		WITH joins AS (
			SELECT DISTINCT room_id, actor_ref FROM funnel_events
			 WHERE event_name = 'participant_joined' AND actor_type = 'agent'
			   AND actor_ref IS NOT NULL AND room_id = ANY($1::uuid[])
		), firsts AS (
			SELECT DISTINCT ON (actor_ref) actor_ref, room_id FROM funnel_events
			 WHERE event_name = 'participant_joined'
			   AND actor_ref IN (SELECT actor_ref FROM joins)
			 ORDER BY actor_ref, occurred_at, id
		)
		SELECT j.room_id::text, j.actor_ref
		  FROM joins j JOIN firsts f ON f.actor_ref = j.actor_ref AND f.room_id = j.room_id`, roomIDs)
	if err != nil {
		LogQueryError(ctx, "ShareAttribution.agents", "funnel_events", err)
		return nil, fmt.Errorf("share attribution new agents: %w", err)
	}
	defer rows.Close()
	byRoom := roomIndex(activated)
	var out []activatedIdentity
	for rows.Next() {
		var room, ref string
		if err := rows.Scan(&room, &ref); err != nil {
			return nil, fmt.Errorf("scan new agent: %w", err)
		}
		a := byRoom[room]
		out = append(out, activatedIdentity{ref: ref, room: room, flow: a.flow, at: *a.activatedAt})
	}
	return out, rows.Err()
}

// newHumans returns the identified humans first seen in an activated room's flow or room,
// and how many activated rooms name no identified human at all (unresolved).
func (r *ShareAttributionRepository) newHumans(ctx context.Context, activated []attributedRoom, roomIDs, flows []string) ([]activatedIdentity, int, error) {
	rows, err := r.pool.Query(ctx, `
		WITH cand AS (
			SELECT DISTINCT actor_ref, COALESCE(flow_id, '') AS flow, COALESCE(room_id::text, '') AS room
			  FROM funnel_events
			 WHERE actor_type = 'human' AND actor_ref IS NOT NULL
			   AND (flow_id = ANY($1::text[]) OR room_id = ANY($2::uuid[]))
		), firsts AS (
			SELECT DISTINCT ON (actor_ref) actor_ref, COALESCE(flow_id, '') AS flow, COALESCE(room_id::text, '') AS room
			  FROM funnel_events
			 WHERE actor_ref IN (SELECT actor_ref FROM cand)
			 ORDER BY actor_ref, occurred_at, id
		)
		SELECT c.actor_ref, c.flow, c.room, f.flow, f.room
		  FROM cand c JOIN firsts f ON f.actor_ref = c.actor_ref`, flows, roomIDs)
	if err != nil {
		LogQueryError(ctx, "ShareAttribution.humans", "funnel_events", err)
		return nil, 0, fmt.Errorf("share attribution new humans: %w", err)
	}
	defer rows.Close()

	identified := map[string]bool{} // activated room -> it names an identified human
	seen := map[string]bool{}
	var out []activatedIdentity
	for rows.Next() {
		var ref, flow, room, firstFlow, firstRoom string
		if err := rows.Scan(&ref, &flow, &room, &firstFlow, &firstRoom); err != nil {
			return nil, 0, fmt.Errorf("scan human: %w", err)
		}
		for _, a := range activated {
			if !(flow != "" && flow == a.flow) && room != a.room {
				continue
			}
			identified[a.room] = true
			isFirst := (firstFlow != "" && firstFlow == a.flow) || firstRoom == a.room
			if isFirst && !seen[ref] {
				seen[ref] = true
				out = append(out, activatedIdentity{ref: ref, room: a.room, flow: a.flow, at: *a.activatedAt})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, len(activated) - len(identified), nil
}

// laterRow is one funnel row of an activated identity after its anchor.
type laterRow struct {
	event, room, flow string
	at                time.Time
}

func (r *ShareAttributionRepository) laterActivity(ctx context.Context, ids []activatedIdentity, now time.Time) (map[string][]laterRow, error) {
	out := map[string][]laterRow{}
	if len(ids) == 0 {
		return out, nil
	}
	refs := make([]string, 0, len(ids))
	earliest := ids[0].at
	for _, id := range ids {
		refs = append(refs, id.ref)
		if id.at.Before(earliest) {
			earliest = id.at
		}
	}
	rows, err := r.pool.Query(ctx, `
		SELECT actor_ref, event_name, COALESCE(room_id::text, ''), COALESCE(flow_id, ''), occurred_at
		  FROM funnel_events
		 WHERE actor_ref = ANY($1::text[]) AND occurred_at > $2 AND occurred_at <= $3`, refs, earliest, now)
	if err != nil {
		LogQueryError(ctx, "ShareAttribution.returns", "funnel_events", err)
		return nil, fmt.Errorf("share attribution returns: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ref string
		var row laterRow
		if err := rows.Scan(&ref, &row.event, &row.room, &row.flow, &row.at); err != nil {
			return nil, fmt.Errorf("scan later activity: %w", err)
		}
		out[ref] = append(out[ref], row)
	}
	return out, rows.Err()
}

// countReturns applies one horizon: eligible once anchor+horizon has passed; returned
// when the identity did other work elsewhere within (anchor, anchor+horizon].
func countReturns(ids []activatedIdentity, later map[string][]laterRow, horizon time.Duration, now time.Time, agents bool) ShareReturns {
	var res ShareReturns
	for _, id := range ids {
		end := id.at.Add(horizon)
		if end.After(now) {
			continue
		}
		res.Eligible++
		for _, row := range later[id.ref] {
			if !row.at.After(id.at) || row.at.After(end) {
				continue
			}
			elsewhere := row.room != id.room && (row.flow == "" || row.flow != id.flow)
			if agents {
				elsewhere = elsewhere && row.event == "participant_joined"
			}
			if elsewhere {
				res.Returned++
				break
			}
		}
	}
	return res
}

func roomIndex(rooms []attributedRoom) map[string]attributedRoom {
	out := make(map[string]attributedRoom, len(rooms))
	for _, r := range rooms {
		out[r.room] = r
	}
	return out
}

func shareDefinitions() map[string]string {
	return map[string]string{
		"share_visits":                         "share_visit steps (a public room page opened from a share link), once per browser tab; visits, not people",
		"invitations":                          "share_link_copied steps attributed to a public room or post",
		"referred_visits":                      "share visits plus Try-this arrivals (connection_started with a source)",
		"attributed_rooms":                     "rooms created from a public source room or post (room_created with a source)",
		"origins":                              "distinct public rooms/posts that produced a referred visit or an invitation in the window",
		"activated_origins":                    "origins that are a room which reached its own two-way exchange, or a published post",
		"invite_to_activation":                 "attributed rooms activated / invitations",
		"referred_visit_activation":            "attributed rooms activated / referred visits",
		"referred_visits_per_activated_origin": "referred visits from activated origins / activated origins",
		"k":                                    "referred_visits_per_activated_origin × referred_visit_activation.rate — an experiment metric",
		"new_agent_activations":                "agent identities whose first-ever room join is in an activated attributed room",
		"new_human_activations":                "identified humans first seen in an activated attributed room's flow or room",
		"unresolved_human_activations":         "activated attributed rooms whose rows name no identified human (anonymous; never guessed)",
		"returns":                              "other work elsewhere within 7/28 days of the room's activation; eligible only once the horizon elapsed",
	}
}

func shareCaveats() []string {
	return []string{
		"k is an experiment metric over a short window, not a forecast of exponential growth.",
		"A second agent of the same person counts as an agent identity, never as a new human.",
		"Multiple agent identities may belong to one person.",
		"Share visits from a tab whose storage is blocked may be counted again on reload.",
	}
}
