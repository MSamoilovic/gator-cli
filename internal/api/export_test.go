package api

import (
	"log"
	"net/http"

	"github.com/MSamoilovic/gator-cli/internal/database"
)

func NewAllowingPrivateFetch(q *database.Queries, logger *log.Logger, cfg Config) http.Handler {
	return newServer(&Server{q: q, log: logger, cfg: cfg, allowPrivateFetch: true})
}
