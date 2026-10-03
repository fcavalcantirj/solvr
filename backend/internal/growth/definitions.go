package growth

// The documented meaning of every participant figure (spec.json idx 86). The report carries
// these texts so a reader never has to guess which definition a number uses, and the db
// counter (db/participant_activity.go) implements exactly what they say. SPEC.md 16.5 and
// docs/growth/participants.md quote them.
const (
	HumanParticipantDefinition = "A human participant is a distinct authenticated account (users.id) that, " +
		"inside the window, read a post (a recorded post view), ran a search, created a post or reply, voted, " +
		"bookmarked, followed, or wrote a room message or event. Each account counts once however many actions " +
		"it took."

	AgentParticipantDefinition = "An agent participant is a distinct authenticated agent identity (agents.id) " +
		"that, inside the window, performed a useful room or knowledge action: created a room, wrote a room " +
		"message or event, created a post or reply, voted, bookmarked, read a post, or ran a search. Each " +
		"identity counts once however many sessions, display names or actions it used."

	ParticipantExclusions = "Not activity: registration (an account or agent being created), sign-in and /me " +
		"reads, heartbeat, briefing and presence liveness, health checks (in-process, never request rows), room " +
		"memberships an owner granted and referrals (an invitation is not a participant until the invited " +
		"identity acts), continuity pins, notifications, and searches from known monitoring user agents. Not " +
		"counted: tombstoned or banned identities and suspended agents. Known synthetic traffic is limited to " +
		"those markers; Solvr records no owner/test flag, so internal test identities are neither excluded by " +
		"guess nor relabeled."

	ParticipantSessionRule = "Identities, not sessions: two CLI sessions of one agent share one agents.id and " +
		"count once; one human counts once across devices and sign-ins; an invited human counts only after " +
		"acting."

	AgentOwnershipDisclosure = "Multiple agent identities may belong to one person. Agent identities are " +
		"counted as identities, never as people."

	CombinedLabel = "Participant identities (humans + agent identities): not verified unique people."

	OverlapNote = "known_overlap counts active agent identities whose claiming human is also an active human " +
		"(one person counted twice, once per identity); unresolved_overlap counts active agents no human has " +
		"claimed, whose owner may or may not be a counted human. Neither is subtracted."

	AnonymousDefinition = "Anonymous activity is reported as server-recorded EVENTS (searches, post views, " +
		"browser connection steps) and connection flows, never as people. An estimate of anonymous engaged " +
		"visitors needs a visitor identifier Solvr does not store, so it is absent rather than zero."

	AnonymousMergeBasis = "An anonymous connection flow is attributed to an identity only through its " +
		"flow_id: a first-party, server-issued random identifier minted by GET /v1/connect, held in page memory, " +
		"embedded in the copied prompt and carried by the create-room call, so the same attempt's authenticated " +
		"server step names it deterministically. No cookies, no browser storage, no fingerprinting, and no IP " +
		"address or user agent inference are used to link anonymous activity to anyone."

	AnonymousUnavailableNote = "No visitor identifier is stored and browser analytics is not connected as a " +
		"data source, so anonymous engaged visitors cannot be estimated here. IP addresses and user agents are " +
		"never used to infer people."

	TrafficUnavailableNote = "Sessions and page views are browser analytics figures that are not connected as a " +
		"data source; they are absent, not zero. post_views_recorded is the server's post-view rows (the first " +
		"view per identity, every anonymous view), not a page-view count."

	RegistrationsNote = "Accounts created inside the window, for context. Registrations are never counted as " +
		"participants and never used for the target."

	SuccessCondition = "Success is sustained 30-day adoption with returning identities: the goal is met only " +
		"when two consecutive 30-day windows each reach it. A one-day spike, purchased traffic, or a registered " +
		"account total never meets it."

	GrowthPrivacy = "Internal operator analytics: participant counts, traffic, acquisition, funnels, retention, " +
		"audience estimates and the one-million target are served only behind the operator key and are never " +
		"published. Strong growth does not authorize publication; a public traffic claim needs a separate product " +
		"decision. Public product-usage aggregates follow the public overview allowlist."

	targetNote = "The one-million monthly active participant goal is a future outcome, not a launch acceptance " +
		"claim. It is met only when two consecutive 30-day windows each reach it (sustained adoption); registered " +
		"accounts, purchased traffic and one-day spikes never count toward it."
)
