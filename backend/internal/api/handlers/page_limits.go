package handlers

// The cursor page bounds of the canonical timeline and reply lists, as published in the
// OpenAPI contract (api/openapi_operations.go). They alias the constants the page parsers
// enforce, so the published default and maximum cannot drift from the running API.
const (
	EntryPageDefaultLimit = defaultEntryPageLimit
	EntryPageMaxLimit     = maxEntryPageLimit
	ReplyPageDefaultLimit = defaultReplyPageLimit
	ReplyPageMaxLimit     = maxReplyPageLimit
)
