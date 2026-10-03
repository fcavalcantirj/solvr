#!/usr/bin/env bash
#
# test_migration.sh - Offline tests for the skill's 4.0.0 migration from 3.x.
#
# 4.0.0 removed the 3.x choices of the legacy knowledge model (post <type>, answer, approach,
# get --include, search --type). Each is refused before any request (and before credentials),
# naming what replaces it and `solvr help migrating`; the version the refusals name is the one
# skill.json publishes; the notes name every removed choice; and the installer installs every
# file solvr.sh sources, so an installed skill runs.
#
# A stub `curl` on PATH records every request and answers with canned responses, and serves the
# installer's downloads from this checkout, so no network (and never production) is touched.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${SKILL_ROOT}/.." && pwd)"
SOLVR_SH="${SCRIPT_DIR}/solvr.sh"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

PASSED=0
FAILED=0

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/bin"
cat > "$WORK/bin/curl" <<'STUB'
#!/usr/bin/env bash
# Stub curl. With -o it is the installer's download: a raw.githubusercontent.com URL of the skill
# is served from $SKILL_ROOT (a missing file gets GitHub's "404: Not Found" body, exit 0, as
# `curl -sL` does). Otherwise it logs "METHOD URL" to $STUB_LOG and prints body + "\n" + status.
method="GET"; url=""; output=""
while [ $# -gt 0 ]; do
    case "$1" in
        -X) method="$2"; shift 2 ;;
        -H|-d|-w) shift 2 ;;
        -o) output="$2"; shift 2 ;;
        -s|-L|-sL) shift ;;
        *) url="$1"; shift ;;
    esac
done
if [ -n "$output" ]; then
    rel="${url#https://raw.githubusercontent.com/fcavalcantirj/solvr/main/skill/}"
    if [ "$rel" != "$url" ] && [ -f "$SKILL_ROOT/$rel" ]; then
        cp "$SKILL_ROOT/$rel" "$output"
    else
        printf '404: Not Found' > "$output"
    fi
    exit 0
fi
echo "${method} ${url}" >> "$STUB_LOG"
case "${method} ${url}" in
    "GET "*/v1/search*)
        printf '%s\n%s' '{"data":[],"meta":{"total":0,"method":"fulltext","took_ms":1}}' 200 ;;
    *)
        printf '%s\n%s' '{"error":{"message":"stub: unexpected request"}}' 500 ;;
esac
STUB
chmod +x "$WORK/bin/curl"
export SKILL_ROOT

# fresh_env resets the request log for one test case.
fresh_env() {
    export STUB_LOG="$WORK/log-$1"
    rm -f "$STUB_LOG"
    : > "$STUB_LOG"
}

# run_solvr runs solvr.sh against the stub with an agent key; it sets $out, $err and $code.
run_solvr() {
    code=0
    PATH="$WORK/bin:$PATH" SOLVR_API_KEY="solvr_agentkey" SOLVR_API_URL="http://stub.local/v1" \
        SOLVR_CONFIG_DIR="$WORK/config" "$SOLVR_SH" "$@" < /dev/null > "$WORK/stdout" 2> "$WORK/stderr" || code=$?
    out=$(cat "$WORK/stdout")
    err=$(cat "$WORK/stderr")
}

# run_solvr_nokey runs solvr.sh with no API key anywhere.
run_solvr_nokey() {
    code=0
    env -u SOLVR_API_KEY PATH="$WORK/bin:$PATH" SOLVR_API_URL="http://stub.local/v1" \
        SOLVR_CONFIG_DIR="$WORK/empty-config" "$SOLVR_SH" "$@" < /dev/null > "$WORK/stdout" 2> "$WORK/stderr" || code=$?
    out=$(cat "$WORK/stdout")
    err=$(cat "$WORK/stderr")
}

requests() { wc -l < "$STUB_LOG" | tr -d ' '; }

check() {
    local name="$1" cond="$2" detail="${3:-}"
    echo -n "Testing: ${name}... "
    if [ "$cond" = true ]; then
        echo -e "${GREEN}PASS${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}FAIL${NC}"
        [ -n "$detail" ] && echo "  $detail"
        FAILED=$((FAILED + 1))
    fi
}

is() { if eval "$1"; then echo true; else echo false; fi; }

# json_version FILE prints the version a skill.json publishes.
json_version() { jq -r .version "$1"; }

VERSION=$(json_version "$SKILL_ROOT/skill.json")
NOTES_POINTER="Run 'solvr help migrating'."

echo "========================================="
echo "Solvr skill migration tests (stub curl)"
echo "========================================="

# 1. The version: skill.json and its published copy agree, it is 4.0.0, and solvr.sh prints it.
check "skill.json publishes 4.0.0" "$(is '[ "$VERSION" = 4.0.0 ]')" "skill.json version: $VERSION"
check "the published skill.json carries the same version" \
    "$(is '[ "$(json_version "$REPO_ROOT/frontend/public/skill.json")" = "$VERSION" ]')" \
    "frontend/public/skill.json version: $(json_version "$REPO_ROOT/frontend/public/skill.json")"
for flag in version --version; do
    fresh_env "$flag"
    run_solvr "$flag"
    check "solvr $flag prints the skill.json version" \
        "$(is '[ "$code" -eq 0 ] && [ "$out" = "solvr skill $VERSION" ]')" "exit $code, stdout: $out, stderr: $err"
done

# 2. Every 3.x choice 4.0.0 removed is refused before any request, naming what replaces it and the
#    notes, on stderr only, with and without --json. Each row: name|instead|invocation (tab-separated).
cat > "$WORK/removed-rows" <<'ROWS'
'solvr answer'	answers and approaches are replies: use solvr reply <post_id> <body>	answer p1 Some answer content
'solvr approach'	answers and approaches are replies: use solvr reply <post_id> <body>	approach p1 Some approach strategy
'solvr post <type>'	posts take no type: "problem" is not accepted; use: solvr post "<title>" "<body>" [--tags <tags>]	post problem Some-title Some-body
'--include'	a post's replies are listed by their own command: use solvr replies p1	get p1 --include answers
'--type'	search covers every post	search pool-race --type problem
'--type'	search covers every post	search --type problem pool-race
ROWS
row=0
while IFS=$'\t' read -r name instead invocation; do
    row=$((row + 1))
    want="Error: ${name} was removed in the solvr skill ${VERSION}; ${instead}. ${NOTES_POINTER}"
    for mode in human json; do
        # shellcheck disable=SC2086
        set -- $invocation
        [ "$mode" = json ] && set -- "$@" --json
        fresh_env "removed-$row-$mode"
        run_solvr "$@"
        check "${invocation} (${mode}) exits 1" "$(is '[ "$code" -eq 1 ]')" "exit $code, stderr: $err"
        check "${invocation} (${mode}) names the removal, its replacement and the notes" \
            "$(is '[ "$err" = "$want" ]')" "stderr: $err | want: $want"
        check "${invocation} (${mode}) prints nothing on stdout" "$(is '[ -z "$out" ]')" "stdout: $out"
        check "${invocation} (${mode}) sends nothing" "$(is '[ "$(requests)" -eq 0 ]')" "log: $(cat "$STUB_LOG")"
    done
    # Refused before credentials: with no key anywhere it is still the removal, not "No API key".
    # shellcheck disable=SC2086
    set -- $invocation
    fresh_env "removed-$row-nokey"
    run_solvr_nokey "$@"
    check "${invocation} with no API key is refused as removed" \
        "$(is '[ "$code" -eq 1 ] && [ "$err" = "$want" ] && [ "$(requests)" -eq 0 ]')" "exit $code, stderr: $err"
done < "$WORK/removed-rows"

# 3. Search still runs with its 4.0.0 options and sends no type.
fresh_env search
run_solvr search "pool race" --limit 5 --sort newest
check "search without --type runs" "$(is '[ "$code" -eq 0 ]')" "exit $code, stderr: $err"
check "search sends exactly q, per_page and sort" \
    "$(is '[ "$(cat "$STUB_LOG")" = "GET http://stub.local/v1/search?q=pool%20race&per_page=5&sort=newest" ]')" \
    "log: $(cat "$STUB_LOG")"

# 4. The help offers no removed choice, and lists the notes and the version.
fresh_env help
run_solvr help
search_options=$(echo "$out" | awk '/^SEARCH OPTIONS:/{f=1; next} /^[A-Z ]+:$/{f=0} f')
get_options=$(echo "$out" | awk '/^GET OPTIONS:/{f=1; next} /^[A-Z ]+:$/{f=0} f')
check "help has a SEARCH OPTIONS section" "$(is '[ -n "$search_options" ]')" "help: $out"
check "help SEARCH OPTIONS offers no --type" "$(is '! echo "$search_options" | grep -q -- "--type"')" "$search_options"
check "help GET OPTIONS offers no --include" "$(is '! echo "$get_options" | grep -q -- "--include"')" "$get_options"
check "help examples search with no --type" "$(is '! echo "$out" | grep -E -q -- "solvr search .*--type"')" "help: $out"
check "help offers no answer, approach or typed post command" \
    "$(is '! echo "$out" | grep -E -q "^    (answer|approach) |^    post <type>"')" "help: $out"
check "help lists help migrating" "$(is 'echo "$out" | grep -E -q "^    help migrating +"')" "help: $out"
check "help lists version" "$(is 'echo "$out" | grep -E -q "^    version +"')" "help: $out"

# 5. The notes: `solvr help migrating` names every removed choice, what replaces it, the version,
#    and only commands solvr.sh runs.
fresh_env notes
run_solvr help migrating
notes="$out"
check "help migrating exits 0 and sends nothing" "$(is '[ "$code" -eq 0 ] && [ "$(requests)" -eq 0 ]')" "exit $code, stderr: $err"
check "help migrating is headed with the version" \
    "$(is '[ "$(echo "$notes" | head -1)" = "Migrating from 3.x to ${VERSION}" ]')" "notes: $notes"
for removed in "solvr post <type>" "solvr answer" "solvr approach" "solvr get <id> --include" "solvr search <query> --type" \
    "solvr reply <post_id> <body>" "solvr replies <id>" "ENDPOINT_RETIRED" "error.details.replacement" \
    "POST /v1/questions/{id}/answers" "POST /v1/problems/{id}/approaches"; do
    check "the notes name ${removed}" "$(is 'echo "$notes" | grep -qF -- "$removed"')" "notes: $notes"
done
fresh_env notes-example
run_solvr search pool-race --type problem
example="$err"
check "the notes show the exact refusal search --type prints" \
    "$(is '[ -n "$example" ] && echo "$notes" | grep -qxF -- "    $example"')" "refusal: $example | notes: $notes"
dispatched=$(sed -n '/^main() {/,/^}/p' "$SOLVR_SH" | grep -E '^        [a-z|-]+\)$' | tr -d ' )' | tr '|' '\n')
for cmd in $(echo "$notes" | grep -oE 'solvr [a-z][a-z-]*' | awk '{print $2}' | sort -u); do
    [ "$cmd" = answer ] || [ "$cmd" = approach ] || [ "$cmd" = skill ] && continue
    check "the notes name only commands solvr.sh runs: ${cmd}" "$(is 'echo "$dispatched" | grep -qx "$cmd"')" \
        "solvr.sh dispatches: $(echo "$dispatched" | tr '\n' ' ')"
done

# 6. SKILL.md and its published copy carry the notes with the same refusal.
for doc in "$SKILL_ROOT/SKILL.md" "$REPO_ROOT/frontend/public/skill.md"; do
    check "$(basename "$(dirname "$doc")")/$(basename "$doc") has the notes section" \
        "$(is 'grep -qxF "## Migrating from 3.x to ${VERSION}" "$doc"')" "$doc"
    check "$(basename "$(dirname "$doc")")/$(basename "$doc") shows the exact refusal" \
        "$(is '[ -n "$example" ] && grep -qxF -- "$example" "$doc"')" "refusal: $example"
done

# 7. The installer installs every file solvr.sh sources: the installed skill runs and prints its
#    version and notes (both the source installer and the copy solvr.dev/install.sh serves).
# sourced_files prints each file a solvr*.sh script sources from its own directory.
sourced_files() {
    local pattern='source "[$][{]SCRIPT_DIR[}]/[^"]*"'
    grep -hoE "$pattern" "$SCRIPT_DIR"/solvr*.sh | sed -E 's|.*/||; s|"$||' | sort -u
}
sourced=$(sourced_files)
check "solvr.sh sources its helper files" "$(is '[ -n "$sourced" ]')" "sourced: $sourced"
for installer in "$REPO_ROOT/scripts/install-solvr-skill.sh" "$REPO_ROOT/frontend/public/install.sh"; do
    name="${installer#"$REPO_ROOT"/}"
    home="$WORK/home-$(echo "$name" | tr '/' '-')"
    mkdir -p "$home"
    code=0
    HOME="$home" PATH="$WORK/bin:$PATH" bash "$installer" > "$WORK/install.log" 2>&1 || code=$?
    installed="$home/.claude/skills/solvr/scripts"
    check "${name} runs" "$(is '[ "$code" -eq 0 ]')" "exit $code: $(tail -3 "$WORK/install.log")"
    for file in solvr.sh $sourced; do
        check "${name} installs scripts/${file}" "$(is 'cmp -s "$SCRIPT_DIR/$file" "$installed/$file"')" \
            "installed: $(head -c 60 "$installed/$file" 2>/dev/null)"
    done
    code=0
    installed_version=$(HOME="$home" bash "$installed/solvr.sh" version 2>&1) || code=$?
    check "the skill ${name} installs prints its version" \
        "$(is '[ "$code" -eq 0 ] && [ "$installed_version" = "solvr skill $VERSION" ]')" "exit $code: $installed_version"
done
zipped=$(sed -n '/zip -r/,/-x /p' "$REPO_ROOT/scripts/sync-skill.sh")
for file in solvr.sh $sourced; do
    check "sync-skill.sh zips scripts/${file}" "$(is 'echo "$zipped" | grep -qF "scripts/${file}"')" "$zipped"
done

# 8. What solvr.dev serves is this skill: every copy sync-skill.sh publishes is byte-identical to
#    its source, and solvr-skill.zip (the /skill page's download) holds exactly the files
#    sync-skill.sh zips, each byte-identical, so the downloaded skill offers no removed 3.x choice.
PUBLIC="$REPO_ROOT/frontend/public"
for pair in "skill/SKILL.md skill.md" "skill/HEARTBEAT.md heartbeat.md" "skill/skill.json skill.json" \
    "skill/references/api.md references/api.md" "skill/references/examples.md references/examples.md" \
    "scripts/install-solvr-skill.sh install.sh"; do
    src="${pair% *}"; dest="${pair#* }"
    check "frontend/public/${dest} is ${src}" "$(is 'cmp -s "$REPO_ROOT/$src" "$PUBLIC/$dest"')" \
        "$(diff "$REPO_ROOT/$src" "$PUBLIC/$dest" 2>&1 | head -4)"
done
zip_list=$(echo "$zipped" | grep -vE 'zip -r|-x ' | tr -d ' \\' | grep -v '^$' | sort)
mkdir -p "$WORK/zip"
code=0
unzip -q "$PUBLIC/solvr-skill.zip" -d "$WORK/zip" > "$WORK/unzip.log" 2>&1 || code=$?
check "frontend/public/solvr-skill.zip unzips" "$(is '[ "$code" -eq 0 ]')" "exit $code: $(cat "$WORK/unzip.log")"
zip_has=$(cd "$WORK/zip" && find . -type f | sed 's|^\./||' | sort)
check "solvr-skill.zip holds exactly the files sync-skill.sh zips" "$(is '[ "$zip_has" = "$zip_list" ]')" \
    "zip: $(echo "$zip_has" | tr '\n' ' ') | sync-skill.sh: $(echo "$zip_list" | tr '\n' ' ')"
for file in $zip_list; do
    check "solvr-skill.zip ${file} is skill/${file}" "$(is 'cmp -s "$SKILL_ROOT/$file" "$WORK/zip/$file"')"
done
code=0
zipped_version=$(HOME="$WORK/zip-home" bash "$WORK/zip/scripts/solvr.sh" version 2>&1) || code=$?
check "the skill solvr-skill.zip serves prints its version" \
    "$(is '[ "$code" -eq 0 ] && [ "$zipped_version" = "solvr skill $VERSION" ]')" "exit $code: $zipped_version"

# 9. The guides steer to no removed choice: outside SKILL.md's migration notes, SKILL.md,
#    HEARTBEAT.md, references/examples.md and the installer's banner never run an approach or
#    answer command, ask to start or post an approach, update an approach's status, accept an
#    answer, or search by a legacy type or status; references/api.md's GET /search documents type
#    and status as legacy filters and its examples use neither.
removed_choice='solvr(\.sh)? (approach|answer)( |$)|solvr(\.sh)? post (problem|question|idea)|--type (problem|question|idea)|--include (approaches|answers)'
removed_choice="${removed_choice}"'|(start|starts|post|posts|contribute)( an| the| your)? approach|approach(es)? status'
removed_choice="${removed_choice}"'|update (the |your |stale )?approach|accept (it|if|the answer|an answer)|mark them .?failed'
removed_choice="${removed_choice}"'|type=(problem|question|idea)|status=(open|solved|stuck)'
outside_notes() { awk '/^## Migrating from 3\.x/{skip=1; next} skip && /^---$/{skip=0} !skip' "$1"; }
for file in skill/SKILL.md skill/HEARTBEAT.md skill/references/examples.md scripts/install-solvr-skill.sh; do
    hits=$(outside_notes "$REPO_ROOT/$file" | grep -i -E "$removed_choice" || true)
    check "${file} steers to no removed choice" "$(is '[ -z "$hits" ]')" "$(echo "$hits" | head -6)"
done
search_ref=$(awk '/^### GET \/search/{on=1; print; next} on && /^### /{exit} on' "$SKILL_ROOT/references/api.md")
hits=$(echo "$search_ref" | grep -i -E 'type=(problem|question|idea)|status=(open|solved|stuck)' || true)
check "references/api.md searches by no legacy type or status" "$(is '[ -z "$hits" ]')" "$hits"
for param in type status; do
    row=$(echo "$search_ref" | grep -E "^\| ${param} \|" || true)
    check "references/api.md documents ${param} as a legacy filter" "$(is 'echo "$row" | grep -q "Legacy"')" "$row"
done

echo ""
echo "Passed: ${PASSED}  Failed: ${FAILED}"
[ "$FAILED" -eq 0 ]
