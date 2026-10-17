package api

import (
	"strings"
	"testing"
)

// The two senders of SPEC.md 16.6. A broadcast goes from FROM_EMAIL (noreply@solvr.dev when it is
// unset); a notice only from FROM_EMAIL_TRANSACTIONAL. Without that address, or with one that is
// not a bare address, there is no notice sender at all and what is missing is reported: the notice
// path can never reach the marketing sender.

type fromAddresser interface{ FromAddress() string }

// fromOf is the address a sender sends from.
func fromOf(t *testing.T, sender any) string {
	t.Helper()
	s, ok := sender.(fromAddresser)
	if !ok {
		t.Fatalf("%T does not say its address", sender)
	}
	return s.FromAddress()
}

type replyToer interface{ ReplyTo() string }

// replyToOf is the Reply-To a sender sends, "" for none.
func replyToOf(t *testing.T, sender any) string {
	t.Helper()
	s, ok := sender.(replyToer)
	if !ok {
		t.Fatalf("%T does not say its Reply-To", sender)
	}
	return s.ReplyTo()
}

func envOf(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestEmailSendersFromEnv(t *testing.T) {
	t.Run("no Resend key: neither sender", func(t *testing.T) {
		broadcast, notice, missing := emailSendersFromEnv(envOf(map[string]string{"FROM_EMAIL_TRANSACTIONAL": "notices@solvr.dev"}))
		if broadcast != nil || notice != nil {
			t.Errorf("senders %v %v, want none", broadcast, notice)
		}
		if missing == "" {
			t.Error("nothing reported missing")
		}
	})

	t.Run("no transactional address: no notice sender, and it is reported", func(t *testing.T) {
		broadcast, notice, missing := emailSendersFromEnv(envOf(map[string]string{"RESEND_API_KEY": "re_test"}))
		if broadcast == nil {
			t.Fatal("no broadcast sender")
		}
		if got := fromOf(t, broadcast); got != "noreply@solvr.dev" {
			t.Errorf("broadcast from %q, want the default noreply@solvr.dev", got)
		}
		if notice != nil {
			t.Errorf("a notice sender exists without FROM_EMAIL_TRANSACTIONAL: from %q", fromOf(t, notice))
		}
		if !strings.Contains(missing, "FROM_EMAIL_TRANSACTIONAL") {
			t.Errorf("missing = %q, want it to name FROM_EMAIL_TRANSACTIONAL", missing)
		}
	})

	t.Run("an address that is not bare: no notice sender", func(t *testing.T) {
		for _, bad := range []string{"Solvr <notices@solvr.dev>", "notices", "a@b.c, d@e.f", " "} {
			_, notice, missing := emailSendersFromEnv(envOf(map[string]string{"RESEND_API_KEY": "re_test", "FROM_EMAIL_TRANSACTIONAL": bad}))
			if notice != nil {
				t.Errorf("%q: a notice sender exists", bad)
			}
			if !strings.Contains(missing, "FROM_EMAIL_TRANSACTIONAL") {
				t.Errorf("%q: missing = %q", bad, missing)
			}
		}
	})

	t.Run("both set: each sender has its own address", func(t *testing.T) {
		broadcast, notice, missing := emailSendersFromEnv(envOf(map[string]string{"RESEND_API_KEY": "re_test",
			"FROM_EMAIL": "news@solvr.dev", "FROM_EMAIL_TRANSACTIONAL": "notices@solvr.dev"}))
		if broadcast == nil || notice == nil {
			t.Fatalf("senders %v %v, want both", broadcast, notice)
		}
		if got := fromOf(t, broadcast); got != "news@solvr.dev" {
			t.Errorf("broadcast from %q", got)
		}
		if got := fromOf(t, notice); got != "notices@solvr.dev" {
			t.Errorf("notice from %q", got)
		}
		if missing != "" {
			t.Errorf("missing = %q, want none", missing)
		}
	})

	t.Run("BROADCAST_REPLY_TO: replies to a broadcast go there, a notice carries none", func(t *testing.T) {
		broadcast, notice, _ := emailSendersFromEnv(envOf(map[string]string{"RESEND_API_KEY": "re_test",
			"FROM_EMAIL_TRANSACTIONAL": "hello@solvr.dev", "BROADCAST_REPLY_TO": "hello@solvr.dev"}))
		if broadcast == nil || notice == nil {
			t.Fatalf("senders %v %v, want both", broadcast, notice)
		}
		if got := fromOf(t, broadcast); got != "noreply@solvr.dev" {
			t.Errorf("broadcast from %q, want noreply@solvr.dev", got)
		}
		if got := replyToOf(t, broadcast); got != "hello@solvr.dev" {
			t.Errorf("broadcast Reply-To %q, want hello@solvr.dev", got)
		}
		if got := replyToOf(t, notice); got != "" {
			t.Errorf("notice Reply-To %q, want none", got)
		}
	})

	t.Run("unset, or not one bare address: no Reply-To, and broadcasts still send", func(t *testing.T) {
		for _, value := range []string{"", " ", "Solvr <hello@solvr.dev>", "hello", "a@b.c, d@e.f", " hello@solvr.dev"} {
			broadcast, _, _ := emailSendersFromEnv(envOf(map[string]string{"RESEND_API_KEY": "re_test", "BROADCAST_REPLY_TO": value}))
			if broadcast == nil {
				t.Fatalf("%q: no broadcast sender", value)
			}
			if got := replyToOf(t, broadcast); got != "" {
				t.Errorf("%q: broadcast Reply-To %q, want none", value, got)
			}
		}
	})
}
