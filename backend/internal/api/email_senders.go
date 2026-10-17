package api

import (
	"strings"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// emailSendersFromEnv builds the two senders of SPEC.md 16.6 from the environment. A broadcast
// goes from FROM_EMAIL (noreply@solvr.dev when unset). A notice goes only from
// FROM_EMAIL_TRANSACTIONAL, a bare address: without one there is no notice sender at all, so a
// notice can never go from the marketing sender. missing says what is not configured, for the log.
func emailSendersFromEnv(getenv func(string) string) (broadcast, notice handlers.EmailSender, missing string) {
	key := getenv("RESEND_API_KEY")
	if key == "" {
		return nil, nil, "RESEND_API_KEY is not set: no email is sent"
	}
	from := getenv("FROM_EMAIL")
	if from == "" {
		from = "noreply@solvr.dev"
	}
	broadcast = services.NewResendClient(key, from)

	transactional := getenv("FROM_EMAIL_TRANSACTIONAL")
	if !handlers.IsBareAddress(transactional) {
		if strings.TrimSpace(transactional) == "" {
			return broadcast, nil, "FROM_EMAIL_TRANSACTIONAL is not set: notices are refused"
		}
		return broadcast, nil, "FROM_EMAIL_TRANSACTIONAL is not one bare address: notices are refused"
	}
	return broadcast, services.NewResendClient(key, transactional), ""
}
