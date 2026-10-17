package api

import (
	"strings"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// emailSendersFromEnv builds the two senders of SPEC.md 16.6 from the environment. A broadcast
// goes from FROM_EMAIL (noreply@solvr.dev when unset), and replies to it go to
// BROADCAST_REPLY_TO when that is one bare address; any other value is ignored, so a bad setting
// never stops broadcasts. A notice goes only from FROM_EMAIL_TRANSACTIONAL, a bare address:
// without one there is no notice sender at all, so a notice can never go from the marketing
// sender. missing says what is not configured, for the log.
func emailSendersFromEnv(getenv func(string) string) (broadcast, notice handlers.EmailSender, missing string) {
	key := getenv("RESEND_API_KEY")
	if key == "" {
		return nil, nil, "RESEND_API_KEY is not set: no email is sent"
	}
	from := getenv("FROM_EMAIL")
	if from == "" {
		from = "noreply@solvr.dev"
	}
	broadcastClient := services.NewResendClient(key, from)
	if replyTo := getenv("BROADCAST_REPLY_TO"); handlers.IsBareAddress(replyTo) {
		broadcastClient.SetReplyTo(replyTo)
	}
	broadcast = broadcastClient

	transactional := getenv("FROM_EMAIL_TRANSACTIONAL")
	if !handlers.IsBareAddress(transactional) {
		if strings.TrimSpace(transactional) == "" {
			return broadcast, nil, "FROM_EMAIL_TRANSACTIONAL is not set: notices are refused"
		}
		return broadcast, nil, "FROM_EMAIL_TRANSACTIONAL is not one bare address: notices are refused"
	}
	return broadcast, services.NewResendClient(key, transactional), ""
}
