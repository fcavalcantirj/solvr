package middleware

import "context"

type inProcessCallKey struct{}

// WithInProcessCall marks a request the API dispatches to its own router: POST /v1/mcp runs
// each tool call as such a request. Only the API can set it (it lives in the context, not in
// a header), and APIUsage does not count it: the MCP call was already counted at the boundary.
func WithInProcessCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, inProcessCallKey{}, true)
}

// IsInProcessCall reports whether WithInProcessCall marked the request.
func IsInProcessCall(ctx context.Context) bool {
	marked, _ := ctx.Value(inProcessCallKey{}).(bool)
	return marked
}
