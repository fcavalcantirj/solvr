# Acquisition planning model (idx 90)

**Report:** `GET /admin/growth/model?month=YYYY-MM` (operator key only; defaults to the last complete month;
SPEC.md 16.5). **Source of truth:** `backend/internal/growth/model.go` (arithmetic, scenarios, bottleneck) and
`backend/internal/db/acquisition_model.go` (observed flows).

## The model

```
A_next = A_current × monthly_retention + new_activated + reactivated − duplicates
```

This is illustrative. The single retention term is replaced by **measured cohort survival** as cohorts accrue:
the report carries each population's monthly cohorts (identities grouped by their first month) and the survival
curve measured from them. `ProjectWithCohorts` projects from that curve instead of one rate.

**Worked arithmetic** (computed in the report, all inputs hypothetical): retaining 80 % of 1,000,000 participants
needs **200,000** new or reactivated participants a month just to stay flat. At a 10 % qualified-visit activation
rate, that means **2,000,000** qualified visits a month.

## Observed flows (per population, never mixed)

For one calendar month (UTC), every active identity falls in exactly one bucket, so nobody is counted twice:

| Bucket | Meaning |
|---|---|
| retained | active this month and last month |
| new | first qualifying action ever is this month |
| reactivated | active this month, not last month, active some month before |

`active = retained + new + reactivated`. `measured_retention = retained / previous_active`. The overlap
adjustment (`known_overlap`) counts agents active this month whose claiming human is also active, the same person
counted under both populations. Activity means the idx 86 qualifying actions.

## Scenarios (hypothetical sensitivity inputs)

Each population, humans and agents separately, gets three scenarios. The starting level, new activations and
reactivations are the **observed** month. The percentages are **hypothetical** and labeled `hypothetical`:

| Scenario | Monthly retention | Qualified-visit activation rate |
|---|---|---|
| conservative | 50 % | 5 % |
| base | 65 %, or the measured retention once ≥ 30 identities were active the month before (`observed`) | 10 % |
| optimistic | 80 % | 15 % |

Each scenario reports a 12-month projection, its steady state, the qualified visits its observed inflow implies,
and the qualified visits it would need to stay flat.

## Channels

SEO, public-room sharing, agent-ecosystem referrals and direct traffic are compared by **retained activations and
cost**, never by visits or downloads. **Public-room sharing** is measured from the idx 88 share attribution for
the month: activated rooms attributed to a public room or post, and the identities they activated that returned
within 28 days. SEO, agent-ecosystem referrals and direct traffic have **no recorded source** and stay
`not_yet_measurable`. No cost source is connected. **Paid acquisition stays off** until retention is measured from cohorts
and the cost per retained activation is known for the channel.

## Bottleneck

The report reads the current bottleneck from the stage gates (idx 89) at the month's end, in order:

1. **capacity**: core-service reliability
2. **connection success**: gate A
3. **repeat usage**: gate B
4. otherwise **reach**

A check that cannot be judged yet stops the reading there, and the report names it as missing.

## Monthly review

Once a month, read the report and work through its `review.checklist`. Record the outcome in the **private**
operator plan, never here:

```
<YYYY-MM> review — bottleneck: <...>; inputs replaced by observed: <...>; base projection vs observed: <...>;
next action: <...>; unmet targets: <...>
```
