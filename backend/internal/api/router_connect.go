package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountConnectRoutes registers the public start-flow contract.
//
//	GET /v1/connect          -- the sentence a visitor copies (text and segments) for
//	                            every use case, with the intent and visibility chosen.
//	GET /v1/connect/examples -- the three example sentences the guides and the home
//	                            page show. Both public, no auth: a visitor with no
//	                            account copies a working sentence.
//
// /connect, the index panel, the guides and the home cards read these endpoints, which
// is what keeps them from drifting apart.
//
// Kept out of router.go, which is already over the file-size limit.
func mountConnectRoutes(r chi.Router, pool *db.Pool) {
	if pool == nil {
		return
	}

	connectHandler := handlers.NewConnectHandler(db.NewRoomRepository(pool))
	// Enable seeding a start flow from a published post (?post=<id>). The lookup only
	// resolves publicly readable posts, so protected content can never seed the contract.
	connectHandler.SetPostLookup(db.NewPostRepository(pool))
	// Enable "Try this workflow" (?from_room=<slug>): only a public room's task structure
	// is read, and it is scrubbed of credentials and private-room links before it is served.
	connectHandler.SetRoomSourceLookup(db.NewRoomRepository(pool))
	r.Get("/v1/connect", connectHandler.GetConnect)
	r.Get("/v1/connect/examples", connectHandler.GetConnectExamples)
}
