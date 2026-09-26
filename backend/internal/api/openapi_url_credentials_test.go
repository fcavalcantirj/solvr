package api

import (
	"testing"
)

// idx 75 step 5: the contract says where a credential may travel, so a client author does
// not learn it from a 400.
func TestOpenAPIConventions_DocumentsThatRoomRoutesRefuseCredentialsInTheURL(t *testing.T) {
	conv := at(t, servedSpec(t), "x-solvr-conventions")

	sameSet(t, "refused URL credential parameters",
		strings_(t, at(t, conv, "credential_transport", "url_credentials", "refused_parameters")),
		[]string{"token", "access_token"})
	sameSet(t, "routes that refuse them",
		strings_(t, at(t, conv, "credential_transport", "url_credentials", "routes")),
		[]string{"/v1/rooms/*", "/r/{slug}/*"})
	if at(t, conv, "credential_transport", "url_credentials", "status") != float64(400) ||
		at(t, conv, "credential_transport", "url_credentials", "code") != "VALIDATION_ERROR" {
		t.Error("a credential in the URL is answered 400 VALIDATION_ERROR")
	}
}
