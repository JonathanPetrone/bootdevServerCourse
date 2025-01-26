package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/joho/godotenv"
	"github.com/jonathanpetrone/bootdevServerCourse/internal/database"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	database       *database.Queries
	secret         string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("couldn't load env variable")
	}

	jwtSecret := os.Getenv("SECRET")
	if jwtSecret == "" {
		log.Fatal("SECRET environment variable is not set")
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("couldnt open database")
	}

	dbQueries := database.New(db)

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	apiCfg := &apiConfig{
		database: dbQueries,
		secret:   jwtSecret,
	}

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
	mux.Handle("GET /admin/metrics", http.HandlerFunc(apiCfg.NumOfRequests))
	mux.Handle("POST /admin/resetmetrics", http.HandlerFunc(apiCfg.ResetRequests))
	mux.Handle("POST /admin/reset", http.HandlerFunc(apiCfg.ResetUsers))
	mux.Handle("/assets", http.FileServer(http.Dir("./assets")))
	mux.Handle("GET /api/healthz", http.HandlerFunc(Readiness))
	mux.Handle("PUT /api/users", http.HandlerFunc(apiCfg.ChangeEmailAndPassword))
	mux.Handle("POST /api/users", http.HandlerFunc(apiCfg.AddUser))
	mux.Handle("GET /api/chirps", http.HandlerFunc(apiCfg.GetChirps))
	mux.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(apiCfg.GetChirp))
	mux.Handle("DELETE /api/chirps/{chirpID}", http.HandlerFunc(apiCfg.DeleteChirp))
	mux.Handle("POST /api/chirps", http.HandlerFunc(apiCfg.CreateChirp))
	mux.Handle("POST /api/login", http.HandlerFunc(apiCfg.LoginUser))
	mux.Handle("POST /api/refresh", http.HandlerFunc(apiCfg.RefreshToken))
	mux.Handle("POST /api/revoke", http.HandlerFunc(apiCfg.RevokeToken))
	log.Printf("Starting server on %s", server.Addr)
	err = server.ListenAndServe()

	if err != nil {
		log.Printf("Server error: %v", err)
		log.Fatal(err)
	}
}
