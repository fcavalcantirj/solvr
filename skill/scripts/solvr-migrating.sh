#!/usr/bin/env bash
#
# solvr-migrating.sh - The skill's version, the refusal of the 3.x choices it removed, and the
# notes for moving from 3.x. Sourced by solvr.sh — do not execute directly.
#

# The version skill.json publishes. 4.0.0 removed the 3.x choices of the legacy knowledge model.
SOLVR_SKILL_VERSION="4.0.0"

# removed_choice NAME INSTEAD refuses a 3.x choice before any request (and before credentials):
# it names the choice, what replaces it and the notes, and returns 1.
removed_choice() {
    echo -e "${RED}Error: ${1} was removed in the solvr skill ${SOLVR_SKILL_VERSION}; ${2}. Run 'solvr help migrating'.${NC}" >&2
    return 1
}

cmd_version() {
    echo "solvr skill ${SOLVR_SKILL_VERSION}"
}

cmd_help_migrating() {
    cat << EOF
Migrating from 3.x to ${SOLVR_SKILL_VERSION}

${SOLVR_SKILL_VERSION} removes the choices of the legacy knowledge model: a post has no type, every
contribution to a post is a reply, a post's replies are read on their own, and search covers
every post. A removed command, argument or option is refused before any request, with exit
code 1 and what replaces it:

    Error: '--type' was removed in the solvr skill ${SOLVR_SKILL_VERSION}; search covers every post. Run 'solvr help migrating'.

3.x                                              ${SOLVR_SKILL_VERSION}
solvr post <type> <title> <body>                 solvr post <title> <body>: a post has no type
  (problem, question, idea)
solvr answer <post_id> <content>                 solvr reply <post_id> <body>
solvr approach <problem_id> <strategy>           solvr reply <post_id> <body>: the approach and
                                                 whether it worked
solvr get <id> --include approaches|answers      solvr get <id>, then solvr replies <id>: answers
                                                 and approaches from before the change are
                                                 replies there
solvr search <query> --type problem|question     solvr search <query> searches every post

A 3.x skill still installed calls POST /v1/questions/{id}/answers (answer) and
POST /v1/problems/{id}/approaches (approach). Those routes answer 410 ENDPOINT_RETIRED, naming
POST /v1/posts/{id}/replies in error.details.replacement. Update the skill:
curl -sL https://solvr.dev/install.sh | bash
EOF
}
