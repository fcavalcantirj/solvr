package handlers

import "encoding/json"

// MCPVersion is the version POST /v1/mcp's initialize answers. 2.0.0 removed the tools and
// arguments of the legacy knowledge model: a 1.x call that still uses one is refused before any
// request, naming what replaces it, instead of failing as an unknown tool or running without the
// choice it asked for (SPEC.md 18.2, "Migrating /v1/mcp from 1.x to 2.0.0"). The npm
// @solvr/mcp-server removed the same ones, with the same replacements (mcp-server/src/removed.ts).
const MCPVersion = "2.0.0"

// mcpRemovedTools are the removed tools, and what replaces each.
var mcpRemovedTools = map[string]string{
	"solvr_answer": "use solvr_reply with post_id and body: answers and approaches are replies",
}

// mcpRemovedArguments are the removed arguments of each tool, and what replaces each.
var mcpRemovedArguments = map[string]map[string]string{
	"solvr_search": {"type": "search covers every post"},
	"solvr_post":   {"type": "a post has no type"},
	"solvr_get": {
		"include": "solvr_get shows the post with its first replies and solvr_replies pages through all of them: answers and approaches from before the change are replies there",
	},
}

func mcpRemovedText(name, instead string) string {
	return name + " was removed in /v1/mcp " + MCPVersion + "; " + instead +
		`. See "Migrating /v1/mcp from 1.x to ` + MCPVersion + `" in SPEC.md 18.2.`
}

// mcpRemovedChoice is why a call of tool with args uses a removed tool or argument; ok is false
// when it uses none. A null argument is not given.
func mcpRemovedChoice(tool string, args map[string]interface{}) (text string, ok bool) {
	if instead, removed := mcpRemovedTools[tool]; removed {
		return mcpRemovedText("'"+tool+"'", instead), true
	}
	for argument, instead := range mcpRemovedArguments[tool] {
		if value := args[argument]; value != nil {
			given, _ := json.Marshal(value)
			return mcpRemovedText("The '"+argument+"' argument of "+tool, instead+" (it was given "+string(given)+")"), true
		}
	}
	return "", false
}
