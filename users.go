package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/google/uuid"
	"github.com/jonathanpetrone/bootdevServerCourse/internal/auth"
	hash "github.com/jonathanpetrone/bootdevServerCourse/internal/auth"
	"github.com/jonathanpetrone/bootdevServerCourse/internal/database"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type LoginForUser struct {
	Password         string `json:"password"`
	Email            string `json:"email"`
	ExpiresInSeconds *int   `json:"expires_in_seconds,omitempty"`
}

type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
}

func (cfg *apiConfig) ResetUsers(rw http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		http.Error(rw, "Forbidden", http.StatusForbidden)
		return
	}

	// Now safely proceed to delete the users
	err := cfg.database.DeleteAllUsers(r.Context())
	if err != nil {
		http.Error(rw, "Failed to reset users", http.StatusInternalServerError)
		return
	}

	rw.Write([]byte("resetted users"))
}

func (apiCfg *apiConfig) AddUser(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json") // Set Content-Type for response

	// Decode incoming JSON into the `User` struct
	req := CreateUserRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		// If JSON decoding fails, return InternalServerError
		log.Printf("Error decoding parameters: %s", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	if req.Password == "" {
		log.Printf("Password is required")
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate email format
	_, err = mail.ParseAddress(req.Email)
	if err != nil {
		// If email is invalid, return BadRequest
		log.Printf("Invalid email address: %s", err)
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Invalid email address"))
		return
	}

	// Hash the password
	hashedPassword, err := hash.HashPassword(req.Password)
	if err != nil {
		log.Printf("Error hashing password: %s", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Create the user parameters for database insertion
	createUser := database.CreateUserParams{
		Email:          req.Email,
		HashedPassword: hashedPassword,
	}

	// Attempt to create the user in the database
	user, err := apiCfg.database.CreateUser(r.Context(), createUser)
	if err != nil {
		// Detect duplicate email error
		if isDuplicateKeyError(err) { // Check for unique key violation
			log.Printf("User already exists with email: %s", createUser.Email)
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("User already exists"))
			return
		}

		// Handle all other errors
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

func (apiCfg *apiConfig) LoginUser(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	loginRequest := LoginForUser{}
	if err := json.NewDecoder(r.Body).Decode(&loginRequest); err != nil {
		resp := errorResponse{Error: "Invalid JSON payload"}
		writeJSONResponse(rw, http.StatusBadRequest, resp)
		return
	}

	if loginRequest.Email == "" || loginRequest.Password == "" {
		resp := errorResponse{Error: "Email and password are required"}
		writeJSONResponse(rw, http.StatusBadRequest, resp)
		return
	}

	user, err := apiCfg.database.GetUser(r.Context(), loginRequest.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			resp := errorResponse{Error: "Incorrect email or password"}
			writeJSONResponse(rw, http.StatusUnauthorized, resp)
			return
		}
		resp := errorResponse{Error: "Internal server error"}
		writeJSONResponse(rw, http.StatusInternalServerError, resp)
		return
	}

	if err := auth.CheckPasswordHash(loginRequest.Password, user.HashedPassword); err != nil {
		resp := errorResponse{Error: "Incorrect email or password"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}

	// Handle expiration time
	expiresIn := 3600 // default 1 hour in seconds
	if loginRequest.ExpiresInSeconds != nil {
		requestedSeconds := *loginRequest.ExpiresInSeconds
		if requestedSeconds > 0 && requestedSeconds <= 3600 {
			expiresIn = requestedSeconds
		}
	}

	// Create token with expiration
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiresIn) * time.Second)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})

	// Sign the token
	tokenString, err := token.SignedString([]byte(apiCfg.secret))
	if err != nil {
		resp := errorResponse{Error: "Error creating token"}
		writeJSONResponse(rw, http.StatusInternalServerError, resp)
		return
	}

	response := LoginResponse{
		ID:        user.ID.String(),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
		Token:     tokenString,
	}

	writeJSONResponse(rw, http.StatusOK, response)
}
