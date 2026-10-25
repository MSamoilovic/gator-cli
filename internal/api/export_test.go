package api

import (
	"log"
	"net/http"

	"github.com/MSamoilovic/gator-cli/internal/database"
)

func NewAllowingPrivateFetch(q *database.Queries, logger *log.Logger) http.Handler {
	return newServer(&Server{q: q, log: logger, allowPrivateFetch: true})
}
