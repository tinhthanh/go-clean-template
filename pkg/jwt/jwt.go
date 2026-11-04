package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// ErrUnexpectedSigningMethod is returned when the JWT signing method is not expected.
var ErrUnexpectedSigningMethod = errors.New("unexpected signing method")

// Manager handles JWT token generation and parsing.
type Manager struct {
	secret   string
	duration time.Duration
}

// New -.
func New(secret string, duration time.Duration) *Manager {
	return &Manager{
		secret:   secret,
		duration: duration,
	}
}

// GenerateToken creates a new JWT token for the given user ID.
func (m *Manager) GenerateToken(userID string) (string, error) {
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(m.duration)),
	})

	tokenString, err := token.SignedString([]byte(m.secret))
	if err != nil {
		return "", fmt.Errorf("jwt - GenerateToken - token.SignedString: %w", err)
	}

	return tokenString, nil
}

// ParseToken validates a JWT token and returns the user ID.
func (m *Manager) ParseToken(tokenString string) (string, error) {
	token, err := jwtlib.Parse(tokenString, func(token *jwtlib.Token) (any, error) {
		if _, ok := token.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnexpectedSigningMethod, token.Header["alg"])
		}

		return []byte(m.secret), nil
	})
	if err != nil {
		return "", fmt.Errorf("jwt - ParseToken - jwtlib.Parse: %w", err)
	}

	sub, err := token.Claims.GetSubject()
	if err != nil {
		return "", fmt.Errorf("jwt - ParseToken - GetSubject: %w", err)
	}

	return sub, nil
}

// ParseExpiry is ParseToken, and when the token ends.
func (m *Manager) ParseExpiry(tokenString string) (string, time.Time, error) {
	claims := &jwtlib.RegisteredClaims{}

	_, err := jwtlib.ParseWithClaims(tokenString, claims, func(token *jwtlib.Token) (any, error) {
		if _, ok := token.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrUnexpectedSigningMethod, token.Header["alg"])
		}

		return []byte(m.secret), nil
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("jwt - ParseExpiry - jwtlib.ParseWithClaims: %w", err)
	}

	var ends time.Time
	if claims.ExpiresAt != nil {
		ends = claims.ExpiresAt.Time
	}

	return claims.Subject, ends, nil
}
