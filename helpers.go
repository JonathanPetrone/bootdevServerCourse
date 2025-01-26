package main

import (
	"encoding/json"
	"net/http"

	"github.com/lib/pq"
)

type errorResponse struct {
	Error string `json:"error"`
}

var BannedWords = []string{"kerfuffle",
	"sharbert",
	"fornax",
}

func isDuplicateKeyError(err error) bool {
	// Check if the error is of type pq.Error
	pqErr, ok := err.(*pq.Error)
	if !ok {
		return false
	}
	// Check if the SQL state matches PostgreSQL's unique_violation code
	return pqErr.Code == "23505"
}

func Readiness(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(200)
	rw.Write([]byte("OK"))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(rw, r)
	})
}

func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(data)
	if err != nil {
		return err
	}
	w.WriteHeader(status)
	w.Write(dat)
	return nil
}
