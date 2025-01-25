package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type CreateUserRequest struct {
	Email string `json:"email"`
}

var BannedWords = []string{"kerfuffle",
	"sharbert",
	"fornax",
}

func Readiness(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(200)
	rw.Write([]byte("OK"))
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
