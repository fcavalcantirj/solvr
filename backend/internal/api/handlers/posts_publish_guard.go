package handlers

import "github.com/fcavalcantirj/solvr/internal/models"

// publishesUnapproved reports whether an edit would publish a post that moderation has not
// approved (rejected, pending review, or an undecided draft). Such an edit goes back through
// moderation instead: an author never self-approves by changing only the status (anti-abuse
// D5a). Family posts are never moderated (BART-154).
func publishesUnapproved(existing, updated models.Post) bool {
	moderation := existing.ModerationState
	if moderation == "" { // not loaded: the status decides it, as the database derives it
		_, moderation = models.DeriveStates(existing.Status)
	}
	if existing.Visibility == models.VisibilityFamily || moderation == models.ModerationApproved {
		return false
	}
	pub, _ := models.DeriveStates(updated.Status)
	return pub == models.PublicationPublished
}
