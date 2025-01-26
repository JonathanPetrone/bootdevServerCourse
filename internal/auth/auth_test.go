package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTCreationAndValidation(t *testing.T) {
	// Create a test UUID and secret
	userID := uuid.New()
	tokenSecret := "your-test-secret"

	// Test cases could include:
	t.Run("valid token", func(t *testing.T) {
		// Create token with 1 hour expiration
		token, err := MakeJWT(userID, tokenSecret, time.Hour)
		if err != nil {
			t.Fatalf("Error creating token: %v", err)
		}

		// Validate the token
		gotUserID, err := ValidateJWT(token, tokenSecret)
		if err != nil {
			t.Fatalf("Error validating token: %v", err)
		}

		if gotUserID != userID {
			t.Errorf("Got user ID %v, want %v", gotUserID, userID)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		// Create token that expires in 1 second
		token, err := MakeJWT(userID, tokenSecret, time.Second)
		if err != nil {
			t.Fatalf("Error creating token: %v", err)
		}

		// Wait for token to expire
		time.Sleep(time.Second * 2)

		// Try to validate expired token
		_, err = ValidateJWT(token, tokenSecret)
		if err == nil {
			t.Error("Expected error for expired token, got nil")
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		// Create token with correct secret
		token, err := MakeJWT(userID, tokenSecret, time.Hour)
		if err != nil {
			t.Fatalf("Error creating token: %v", err)
		}

		// Try to validate with wrong secret
		wrongSecret := "wrong-secret"
		_, err = ValidateJWT(token, wrongSecret)
		if err == nil {
			t.Error("Expected error for wrong secret, got nil")
		}
	})

	t.Run("invalid token string", func(t *testing.T) {
		// Try to validate completely invalid string
		_, err := ValidateJWT("not-a-valid-token", tokenSecret)
		if err == nil {
			t.Error("Expected error for invalid token string, got nil")
		}

		// Try to validate malformed JWT
		_, err = ValidateJWT("header.payload.wrongsignature", tokenSecret)
		if err == nil {
			t.Error("Expected error for malformed JWT, got nil")
		}
	})
}
