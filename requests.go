package main

import (
	"fmt"
	"net/http"
)

func (cfg *apiConfig) NumOfRequests(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html")
	count := cfg.fileserverHits.Load()
	template := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", count)
	rw.Write([]byte(template))
}

func (cfg *apiConfig) ResetRequests(rw http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	rw.Write([]byte("resetted request count"))
}
