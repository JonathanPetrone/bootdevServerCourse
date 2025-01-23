package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}
	mux.Handle("/", http.FileServer(http.Dir(".")))
	log.Printf("Starting server on %s", server.Addr)
	err := server.ListenAndServe()

	if err != nil {
		log.Printf("Server error: %v", err)
		log.Fatal(err)
	}
}
