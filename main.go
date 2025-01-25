package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/jonathanpetrone/bootdevServerCourse/internal/database"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	database       *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(rw, r)
	})
}

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

func (apiCfg *apiConfig) AddUser(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json") // Set Content-Type for response

	// Decode incoming JSON into the `User` struct
	decoder := json.NewDecoder(r.Body)
	u := User{}
	err := decoder.Decode(&u)
	if err != nil {
		// If JSON decoding fails, return InternalServerError
		log.Printf("Error decoding parameters: %s", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Validate email format
	_, err = mail.ParseAddress(u.Email)
	if err != nil {
		// If email is invalid, return BadRequest
		log.Printf("Invalid email address: %s", err)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	// Attempt to create the user in the database
	user, err := apiCfg.database.CreateUser(r.Context(), u.Email)
	if err != nil {
		// If database insertion fails, return InternalServerError
		log.Printf("Error creating user: %s", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Map database user to response user
	newUser := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	// Provide a success case: Set 201 Created status and encode user
	rw.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(rw)
	err = encoder.Encode(newUser)
	if err != nil {
		// If response encoding fails, log but do not reset status
		log.Printf("Error encoding response: %s", err)
	}
}

func (apiCfg *apiConfig) CreateChirp(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	decoder := json.NewDecoder(r.Body)
	c := Chirp{}
	err := decoder.Decode(&c)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		resp := errorResponse{Error: "Invalid JSON payload"}
		writeJSONResponse(rw, 400, resp)
		return
	}

	if c.Body == "" {
		resp := errorResponse{Error: "Chirp body is required"}
		writeJSONResponse(rw, 400, resp)
		return
	}

	if len(c.Body) > 140 {
		resp := errorResponse{Error: "Chirp is too long"}
		writeJSONResponse(rw, 400, resp)
		return
	}

	messageWords := strings.Split((c.Body), " ")

	for i := range messageWords {
		for j := range BannedWords {
			if strings.ToLower(messageWords[i]) == BannedWords[j] {
				messageWords[i] = "****"
			}
		}
	}

	joinedWords := strings.Join(messageWords, " ")

	chirp := database.CreateChirpParams{
		Body:   joinedWords,
		UserID: uuid.NullUUID{UUID: c.UserID, Valid: true}, // Convert c.UserID to NullUUID
	}

	createdChirp, err := apiCfg.database.CreateChirp(r.Context(), chirp)
	if err != nil {
		log.Printf("Error creating chirp in database: %s", err)
		resp := errorResponse{Error: "Failed to save chirp to database"}
		writeJSONResponse(rw, 500, resp)
		return
	}

	resp := Chirp{
		ID:        createdChirp.ID,
		CreatedAt: createdChirp.CreatedAt,
		UpdatedAt: createdChirp.UpdatedAt,
		Body:      createdChirp.Body,
		UserID:    createdChirp.UserID.UUID, // Extract from NullUUID
	}

	if err := writeJSONResponse(rw, 201, resp); err != nil {
		log.Printf("Error writing JSON response: %s", err)
		rw.WriteHeader(500)
		return
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("couldn't load env variable")
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	apiCfg := &apiConfig{
		database: dbQueries,
	}

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
	mux.Handle("GET /admin/metrics", http.HandlerFunc(apiCfg.NumOfRequests))
	mux.Handle("POST /admin/reset", http.HandlerFunc(apiCfg.ResetRequests))
	mux.Handle("POST /api/validate_chirp", http.HandlerFunc(apiCfg.CreateChirp))
	mux.Handle("/assets", http.FileServer(http.Dir("./assets")))
	mux.Handle("GET /api/healthz", http.HandlerFunc(Readiness))
	mux.Handle("POST /api/users", http.HandlerFunc(apiCfg.AddUser))
	mux.Handle("POST /api/chirps", http.HandlerFunc(apiCfg.CreateChirp))
	log.Printf("Starting server on %s", server.Addr)
	err = server.ListenAndServe()

	if err != nil {
		log.Printf("Server error: %v", err)
		log.Fatal(err)
	}
}
