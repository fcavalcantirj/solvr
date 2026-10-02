# Cutover window G2/G5: one-line summary of a cmd/cutover JSON report (never the sampled query texts).
# all_zero is the second-pass criterion: nothing left to change.
{
  run: .run_id[0:8], dry_run,
  post_exceptions, post_states_remapped,
  pending_contributions, pending_after, contribution_exceptions,
  replies_created, progress_notes, unresolved_relations, relations,
  drift_before_after: {
    vote: [.vote_drift_before, .vote_drift_after], room: [.room_drift_before, .room_drift_after],
    view: [.view_drift_before, .view_drift_after], reputation: [.reputation_drift_before, .reputation_drift_after]},
  rebuilt: {vote: .vote_scores_rebuilt, room: .room_activity_rebuilt, view: .view_counts_rebuilt, reputation: .reputation_rebuilt},
  search_documents_pending,
  search: (.search_sample | if . == null then null else {
    queries: (.queries // [] | length), posts_before, posts_after, lost: (.lost_posts // [] | length),
    contributions, contributions_found, missing: (.missing_contributions // [] | length)} end),
  all_zero: ([.post_states_remapped, .replies_created, .progress_notes, ((.relations // {}) | add // 0),
    .pending_contributions, .pending_after,
    .vote_drift_before, .vote_scores_rebuilt, .vote_drift_after, .room_drift_before, .room_activity_rebuilt, .room_drift_after,
    .view_drift_before, .view_counts_rebuilt, .view_drift_after,
    .reputation_drift_before, .reputation_rebuilt, .reputation_drift_after] | all(. == 0))
}
