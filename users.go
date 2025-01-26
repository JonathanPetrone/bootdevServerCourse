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
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
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

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), // always 1 hour
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})

	// Sign the token
	tokenString, err := token.SignedString([]byte(apiCfg.secret))
	if err != nil {
		resp := errorResponse{Error: "Error creating token"}
		writeJSONResponse(rw, http.StatusInternalServerError, resp)
		return
	}

	// Generate refresh token
	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		resp := errorResponse{Error: "Error creating refresh token"}
		writeJSONResponse(rw, http.StatusInternalServerError, resp)
		return
	}

	// Store refresh token in database
	_, err = apiCfg.database.CreateRefreshtoken(r.Context(), database.CreateRefreshtokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(60 * 24 * time.Hour), // 60 days
	})
	if err != nil {
		resp := errorResponse{Error: "Error storing refresh token"}
		writeJSONResponse(rw, http.StatusInternalServerError, resp)
		return
	}

	response := LoginResponse{
		ID:           user.ID.String(),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        tokenString,
		RefreshToken: refreshToken,
	}

	writeJSONResponse(rw, http.StatusOK, response)
}

func (apiCfg *apiConfig) ChangeEmailAndPassword(rw http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		resp := errorResponse{Error: "Authentication required"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}

	userID, err := auth.ValidateJWT(token, apiCfg.secret)
	if err != nil {
		resp := errorResponse{Error: "Invalid token"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}

	user := CreateUserRequest{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&user); err != nil {
		resp := errorResponse{Error: "Invalid request body"}
		writeJSONResponse(rw, http.StatusBadRequest, resp)
		return
	}

	newPassword, err := auth.HashPassword(user.Password)
	if err != nil {
		resp := errorResponse{Error: "Couldn't hash password"}
		writeJSONResponse(rw, http.StatusBadRequest, resp)
		return
	}

	params := database.UpdateUserParams{
		Email:          user.Email,
		HashedPassword: newPassword,
		ID:             userID,
	}

	updatedUser, err := apiCfg.database.UpdateUser(r.Context(), params)
	if err != nil {
		resp := errorResponse{Error: "Couldn't update user"}
		writeJSONResponse(rw, http.StatusBadRequest, resp)
		return
	}

	userResponse := UserResponse{
		ID:        updatedUser.ID.String(),
		Email:     updatedUser.Email,
		CreatedAt: updatedUser.CreatedAt,
	}

	writeJSONResponse(rw, http.StatusOK, userResponse)
}
