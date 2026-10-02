# Cutover window R5: compare two legacy-state.sql snapshots (before = G3, after = the rollback).
#   jq -n --slurpfile a before.json --slurpfile b after.json -f compare-legacy.jq
($a[0]) as $A | ($b[0]) as $B |
{
  counts_changed: [ $A.counts | to_entries[] | select($B.counts[.key] != .value) | {k: .key, before: .value, after: $B.counts[.key]} ],
  legacy_public_contributions: {before: $A.legacy_public_contributions, after: $B.legacy_public_contributions},
  tombstones_equal: ($A.tombstones == $B.tombstones), tombstones: ($B.tombstones | length),
  trigger: {exists: $B.trigger.exists, triggerdef_equal: ($A.trigger.triggerdef == $B.trigger.triggerdef),
            body_equal: ($A.trigger.functiondef == $B.trigger.functiondef)},
  rooms: {ids_slugs_equal: (($A.rooms_list | map({id, slug})) == ($B.rooms_list | map({id, slug}))),
          owners_equal: (($A.rooms_list | map({id, owner_id})) == ($B.rooms_list | map({id, owner_id}))),
          shared_tokens_still_valid: ([ $A.rooms_list[] as $x | $B.rooms_list[] | select(.id == $x.id and .token_md5 == $x.token_md5) ] | length),
          of: ($A.rooms_list | length)},
  agent_tokens: {before: ($A.room_agent_tokens_list | length), after: ($B.room_agent_tokens_list | length),
                 same_hash: ([ $A.room_agent_tokens_list[] as $x | $B.room_agent_tokens_list[] | select(.room_id == $x.room_id and .agent_id == $x.agent_id and .token_md5 == $x.token_md5) ] | length)}
}
