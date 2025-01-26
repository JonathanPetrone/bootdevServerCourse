package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanpetrone/bootdevServerCourse/internal/auth"
	"github.com/jonathanpetrone/bootdevServerCourse/internal/database"
)

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (apiCfg *apiConfig) CreateChirp(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	// Get token from Authorization header
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		resp := errorResponse{Error: "Authentication required"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}

	// Validate JWT and get userID directly
	userID, err := auth.ValidateJWT(token, apiCfg.secret)
	if err != nil {
		resp := errorResponse{Error: "Invalid token"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}

	decoder := json.NewDecoder(r.Body)
	c := Chirp{}
	err = decoder.Decode(&c)
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
		UserID: userID,
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
		UserID:    createdChirp.UserID,
	}

	if err := writeJSONResponse(rw, 201, resp); err != nil {
		log.Printf("Error writing JSON response: %s", err)
		rw.WriteHeader(500)
		return
	}
}

func (apiCfg *apiConfig) GetChirps(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	arrayOfChirps, err := apiCfg.database.GetAllChirps(r.Context())
	if err != nil {
		log.Printf("Error getting Chirps: %s", err)
		rw.WriteHeader(500)
		return
	}

	responseChirps := []Chirp{}

	for _, dbChirp := range arrayOfChirps {

		chirp := Chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		}
		responseChirps = append(responseChirps, chirp)
	}

	rw.WriteHeader(200)
	if err := json.NewEncoder(rw).Encode(responseChirps); err != nil {
		log.Printf("Error encoding response: %v", err)
		return
	}
}

func (apiCfg *apiConfig) GetChirp(rw http.ResponseWriter, r *http.Request) {
	// Call PathValue to get the chirpID from the path as a string
	chirpIDStr := r.PathValue("chirpID")

	// Convert the string to a uuid.UUID
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		// Handle the error appropriately if the ID is not a valid UUID
		http.Error(rw, "Invalid Chirp ID", http.StatusBadRequest)
		return
	}

	// Fetch the chirp from the database using the valid chirpID
	chirp, err := apiCfg.database.GetOneChirp(r.Context(), chirpID)
	if err != nil {
		// Handle the case where the chirp is not found
		http.Error(rw, "Chirp not found", http.StatusNotFound)
		return
	}

	chirpRes := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}

	// If the chirp is found, respond with its data (this part is an example)
	writeJSONResponse(rw, http.StatusOK, chirpRes)
}

func (apiCfg *apiConfig) DeleteChirp(rw http.ResponseWriter, r *http.Request) {
	chirpIDStr := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		resp := errorResponse{Error: "Invalid chirp ID"}
		writeJSONResponse(rw, http.StatusBadRequest, resp)
		return
	}

	// Get token from Authorization header
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		resp := errorResponse{Error: "Authentication required"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}

	// Validate JWT and get userID directly
	userID, err := auth.ValidateJWT(token, apiCfg.secret)
	if err != nil {
		resp := errorResponse{Error: "Invalid token"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}

	chirp, err := apiCfg.database.GetOneChirp(r.Context(), chirpID)
	if err != nil {
		resp := errorResponse{Error: "Chirp not found"}
		writeJSONResponse(rw, http.StatusNotFound, resp)
		return
	}

	// Check if user is the author
	if chirp.UserID != userID {
		resp := errorResponse{Error: "Forbidden"}
		writeJSONResponse(rw, http.StatusForbidden, resp)
		return
	}

	// Delete the chirp
	err = apiCfg.database.DeleteOneChirp(r.Context(), chirpID)
	if err != nil {
		resp := errorResponse{Error: "Failed to delete chirp"}
		writeJSONResponse(rw, http.StatusInternalServerError, resp)
		return
	}

	// Success - return 204 No Content
	rw.WriteHeader(http.StatusNoContent)

}
