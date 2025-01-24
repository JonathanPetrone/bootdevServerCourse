package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(rw, r)
	})
}

func (cfg *apiConfig) NumOfRequests(rw http.ResponseWriter, r *http.Request) {
	count := cfg.fileserverHits.Load()
	rstring := fmt.Sprintf("Hits: %d", count)
	rw.Write([]byte(rstring))
}

func (cfg *apiConfig) ResetRequests(rw http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	rw.Write([]byte("resetted request count"))
}

func main() {
	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	apiCfg := &apiConfig{}

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
	mux.Handle("GET /metrics", http.HandlerFunc(apiCfg.NumOfRequests))
	mux.Handle("POST /reset", http.HandlerFunc(apiCfg.ResetRequests))
	mux.Handle("/assets", http.FileServer(http.Dir("./assets")))
	mux.Handle("GET /healthz", http.HandlerFunc(Readiness))
	log.Printf("Starting server on %s", server.Addr)
	err := server.ListenAndServe()

	if err != nil {
		log.Printf("Server error: %v", err)
		log.Fatal(err)
	}
}
