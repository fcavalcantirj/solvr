package emailutil

import (
	"strings"
	"testing"
)

func TestWrapInBrandedTemplate_ContainsBrandHeader(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Hello</p>", "https://example.com/unsub", "Test note")

	if !strings.Contains(html, "SOLVR_") {
		t.Error("expected branded header with SOLVR_")
	}
	if !strings.Contains(html, "background-color: #0a0a0a") {
		t.Error("expected dark header background")
	}
}

func TestWrapInBrandedTemplate_ContainsInnerContent(t *testing.T) {
	inner := `<h1 style="color: #1a1a1a;">Welcome!</h1><p>Some content here</p>`
	html := WrapInBrandedTemplate(inner, "https://example.com/unsub", "Test note")

	if !strings.Contains(html, "Welcome!") {
		t.Error("expected inner content to be present")
	}
	if !strings.Contains(html, "Some content here") {
		t.Error("expected inner content paragraph to be present")
	}
}

func TestWrapInBrandedTemplate_ContainsFooterNote(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Body</p>", "https://example.com/unsub", "You signed up for Solvr")

	if !strings.Contains(html, "You signed up for Solvr") {
		t.Error("expected footer note to be present")
	}
}

func TestWrapInBrandedTemplate_ContainsUnsubscribeLink(t *testing.T) {
	unsubURL := "https://api.solvr.dev/v1/email/unsubscribe?email=test@example.com&token=abc123"
	html := WrapInBrandedTemplate("<p>Body</p>", unsubURL, "Test note")

	if !strings.Contains(html, unsubURL) {
		t.Error("expected unsubscribe URL to be present")
	}
	if !strings.Contains(html, "Unsubscribe") {
		t.Error("expected Unsubscribe link text")
	}
}

func TestWrapInBrandedTemplate_ContainsTagline(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Body</p>", "https://example.com/unsub", "Test note")

	if !strings.Contains(html, "The knowledge base for developers and AI agents") {
		t.Error("expected Solvr tagline in footer")
	}
}

func TestWrapInBrandedTemplate_HasProperStructure(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Body</p>", "https://example.com/unsub", "Test note")

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected DOCTYPE declaration")
	}
	if !strings.Contains(html, "<html") {
		t.Error("expected html tag")
	}
	if !strings.Contains(html, "</html>") {
		t.Error("expected closing html tag")
	}
	if !strings.Contains(html, "background-color: #f4f4f5") {
		t.Error("expected outer background color #f4f4f5")
	}
	if !strings.Contains(html, "background-color: #ffffff") {
		t.Error("expected content background color #ffffff")
	}
	if !strings.Contains(html, "max-width: 600px") {
		t.Error("expected max-width 600px")
	}
}

func TestWrapInBrandedTemplate_UsesMonospaceFont(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Body</p>", "https://example.com/unsub", "Test note")

	if !strings.Contains(html, "SF Mono") {
		t.Error("expected SF Mono in font stack")
	}
	if !strings.Contains(html, "Fira Code") {
		t.Error("expected Fira Code in font stack")
	}
	if !strings.Contains(html, "Consolas") {
		t.Error("expected Consolas in font stack")
	}
}

func TestWrapInBrandedTemplate_HasInlineStylesOnly(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Body</p>", "https://example.com/unsub", "Test note")

	// Should NOT contain <style> tags (inline styles only for email compatibility)
	if strings.Contains(html, "<style") {
		t.Error("should not contain <style> tags — use inline styles only for email compatibility")
	}
	// Should NOT contain class attributes
	if strings.Contains(html, "class=") {
		t.Error("should not contain class attributes — use inline styles only for email compatibility")
	}
}

func TestWrapInBrandedTemplate_HasTableBasedLayout(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Body</p>", "https://example.com/unsub", "Test note")

	if !strings.Contains(html, `role="presentation"`) {
		t.Error("expected table-based layout with role=presentation")
	}
}

func TestWrapInBrandedTemplate_SettingsURL(t *testing.T) {
	// Notification emails use settings URL instead of HMAC unsubscribe
	settingsURL := "https://solvr.dev/settings/notifications"
	html := WrapInBrandedTemplate("<p>Body</p>", settingsURL, "You asked a question on Solvr")

	if !strings.Contains(html, settingsURL) {
		t.Error("expected settings URL as unsubscribe link")
	}
}

func TestWrapInBrandedTemplate_ContentBorders(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Body</p>", "https://example.com/unsub", "Test note")

	if !strings.Contains(html, "border-left: 1px solid #e4e4e7") {
		t.Error("expected left border on content area")
	}
	if !strings.Contains(html, "border-right: 1px solid #e4e4e7") {
		t.Error("expected right border on content area")
	}
	if !strings.Contains(html, "border-bottom: 1px solid #e4e4e7") {
		t.Error("expected bottom border on content area")
	}
}

func TestWrapInBrandedTemplate_HeaderLetterSpacing(t *testing.T) {
	html := WrapInBrandedTemplate("<p>Body</p>", "https://example.com/unsub", "Test note")

	if !strings.Contains(html, "letter-spacing: 2px") {
		t.Error("expected letter-spacing: 2px on SOLVR_ header")
	}
}
