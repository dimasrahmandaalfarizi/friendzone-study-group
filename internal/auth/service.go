package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/BallSpins/friendzone/internal/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

const SessionTTL = 7 * 24 * time.Hour

// Consumer-defined interfaces (Dependency Inversion)
type UserProvisioner interface {
	SyncGoogleAccountID(ctx context.Context, sub, email, name, picture string) (string, error)
}

type SessionRepository interface {
	Save(ctx context.Context, token, userID string, ttl time.Duration) error
	GetUserID(ctx context.Context, token string) (string, error)
	Delete(ctx context.Context, token string) error
}

type Service struct {
	oauthConfig *oauth2.Config
	clientID    string
	users       UserProvisioner
	sessions    SessionRepository
}

func NewService(cfg *config.Config, users UserProvisioner, sessions SessionRepository) *Service {
	return &Service{
		clientID: cfg.GoogleClientID,
		users:    users,
		sessions: sessions,
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (s *Service) BeginGoogleLogin() (authURL, state, verifier string, err error) {
	state, err = generateSecureToken(32)
	if err != nil {
		return "", "", "", fmt.Errorf("generate state: %w", err)
	}

	verifier = oauth2.GenerateVerifier()
	authURL = s.oauthConfig.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	return authURL, state, verifier, nil
}

func (s *Service) CompleteGoogleLogin(ctx context.Context, code, verifier string) (string, error) {
	token, err := s.oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", fmt.Errorf("exchange authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", errors.New("missing id_token in oauth response")
	}

	payload, err := idtoken.Validate(ctx, rawIDToken, s.clientID)
	if err != nil {
		return "", fmt.Errorf("validate id_token: %w", err)
	}

	emailVerified, _ := payload.Claims["email_verified"].(bool)
	if !emailVerified {
		return "", errors.New("google email is not verified")
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	// 1. Sinkronkan akun user ke PostgreSQL
	userID, err := s.users.SyncGoogleAccountID(ctx, payload.Subject, email, name, picture)
	if err != nil {
		return "", fmt.Errorf("sync user account: %w", err)
	}

	// 2. Buat token sesi baru dan simpan ke Redis dengan TTL
	sessionToken, err := generateSecureToken(32)
	if err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	if err := s.sessions.Save(ctx, sessionToken, userID, SessionTTL); err != nil {
		return "", fmt.Errorf("save session: %w", err)
	}

	return sessionToken, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, sessionToken string) (string, error) {
	return s.sessions.GetUserID(ctx, sessionToken)
}

func (s *Service) Logout(ctx context.Context, sessionToken string) error {
	return s.sessions.Delete(ctx, sessionToken)
}

func generateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}