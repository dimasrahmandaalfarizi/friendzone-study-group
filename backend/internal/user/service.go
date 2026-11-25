package user

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Repository interface {
	UpsertByGoogleSub(ctx context.Context, sub, email, name, picture string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	PatchBasicProfile(ctx context.Context, userID string, in PatchProfileInput) (*User, error)
	UpsertProfile(ctx context.Context, userID string, in UpsertProfileInput, embedding []float32) (*AcademicProfile, error)
	FindUserWithProfileByID(ctx context.Context, id string) (*UserWithProfile, error)
}

type Service struct {
	repo     Repository
	embedder TextEmbedder
}

func NewService(repo Repository, embedder TextEmbedder) *Service {
	return &Service{
		repo:     repo,
		embedder: embedder,
	}
}

func (s *Service) SyncGoogleAccount(ctx context.Context, sub, email, name, picture string) (*User, error) {
	if strings.TrimSpace(sub) == "" || strings.TrimSpace(email) == "" {
		return nil, errors.New("google sub and email are required")
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	return s.repo.UpsertByGoogleSub(ctx, sub, normalizedEmail, strings.TrimSpace(name), picture)
}

func (s *Service) GetProfile(ctx context.Context, userID string) (*User, error) {
	return s.repo.FindByID(ctx, userID)
}

func (s *Service) GetFullProfile(ctx context.Context, userID string) (*UserWithProfile, error) {
	return s.repo.FindUserWithProfileByID(ctx, userID)
}

func (s *Service) PatchBasicProfile(ctx context.Context, userID string, in PatchProfileInput) (*User, error) {
	if in.Name == nil && in.Picture == nil {
		return nil, fmt.Errorf("%w: at least one of name or picture is required", ErrInvalidProfilePatch)
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrInvalidProfilePatch)
		}
		if utf8.RuneCountInString(name) > 255 {
			return nil, fmt.Errorf("%w: name must not exceed 255 characters", ErrInvalidProfilePatch)
		}
		in.Name = &name
	}

	if in.Picture != nil {
		picture := strings.TrimSpace(*in.Picture)
		in.Picture = &picture
	}

	return s.repo.PatchBasicProfile(ctx, userID, in)
}

func (s *Service) UpdateAcademicProfile(ctx context.Context, userID string, in UpsertProfileInput) (*AcademicProfile, error) {
	in.University = strings.TrimSpace(in.University)
	in.Major = strings.TrimSpace(in.Major)
	in.StudyMode = strings.ToLower(strings.TrimSpace(in.StudyMode))
	in.Bio = strings.TrimSpace(in.Bio)

	if in.University == "" || in.Major == "" {
		return nil, errors.New("university and major are required")
	}
	if len(in.TargetSubjects) == 0 {
		return nil, errors.New("at least one target subject is required")
	}
	if !slices.Contains([]string{"online", "in-person", "hybrid"}, in.StudyMode) {
		return nil, errors.New("study_mode must be online, in-person, or hybrid")
	}

	// Gabungkan atribut profil menjadi representasi teks untuk semantic matching
	semanticText := fmt.Sprintf(
		"University: %s. Major: %s. Target Subjects: %s. Preferred Study Mode: %s. Bio: %s",
		in.University,
		in.Major,
		strings.Join(in.TargetSubjects, ", "),
		in.StudyMode,
		in.Bio,
	)

	vector, err := s.embedder.Embed(ctx, semanticText)
	if err != nil {
		return nil, fmt.Errorf("generate profile embedding: %w", err)
	}
	if len(vector) != 768 {
		return nil, fmt.Errorf("generate profile embedding: expected 768 dimensions, got %d", len(vector))
	}

	return s.repo.UpsertProfile(ctx, userID, in, vector)
}
