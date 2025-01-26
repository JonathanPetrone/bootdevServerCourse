package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func (apiCfg *apiConfig) RefreshToken(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	// Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		resp := errorResponse{Error: "Authorization header is required"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}
	refreshToken := strings.TrimPrefix(authHeader, "Bearer ")

	// Get user from refresh token
	user, err := apiCfg.database.GetUserFromRefreshToken(r.Context(), refreshToken)
	if err != nil {
		resp := errorResponse{Error: "Invalid refresh token"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}

	// Create new access token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})

	// Sign the token
	tokenString, err := token.SignedString([]byte(apiCfg.secret))
	if err != nil {
		resp := errorResponse{Error: "Error creating token"}
		writeJSONResponse(rw, http.StatusInternalServerError, resp)
		return
	}

	// Return new access token
	response := struct {
		Token string `json:"token"`
	}{
		Token: tokenString,
	}

	writeJSONResponse(rw, http.StatusOK, response)
}

func (apiCfg *apiConfig) RevokeToken(rw http.ResponseWriter, r *http.Request) {
	// Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		resp := errorResponse{Error: "Authorization header is required"}
		writeJSONResponse(rw, http.StatusUnauthorized, resp)
		return
	}
	refreshToken := strings.TrimPrefix(authHeader, "Bearer ")

	// Revoke the token
	err := apiCfg.database.RevokeRefreshToken(r.Context(), refreshToken)
	if err != nil {
		resp := errorResponse{Error: "Could not revoke token"}
		writeJSONResponse(rw, http.StatusInternalServerError, resp)
		return
	}

	// Return 204 No Content
	rw.WriteHeader(http.StatusNoContent)
}
