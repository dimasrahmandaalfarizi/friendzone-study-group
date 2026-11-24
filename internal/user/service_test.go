package user

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type profilePatchRepository struct {
	input    PatchProfileInput
	called   bool
	response *User
}

func (r *profilePatchRepository) UpsertByGoogleSub(context.Context, string, string, string, string) (*User, error) {
	return nil, nil
}

func (r *profilePatchRepository) FindByID(context.Context, string) (*User, error) {
	return nil, nil
}

func (r *profilePatchRepository) PatchBasicProfile(_ context.Context, _ string, in PatchProfileInput) (*User, error) {
	r.input = in
	r.called = true
	return r.response, nil
}

func (r *profilePatchRepository) UpsertProfile(context.Context, string, UpsertProfileInput, []float32) (*AcademicProfile, error) {
	return nil, nil
}

func (r *profilePatchRepository) FindUserWithProfileByID(context.Context, string) (*UserWithProfile, error) {
	return nil, nil
}

func TestPatchBasicProfileValidation(t *testing.T) {
	service := NewService(&profilePatchRepository{}, nil)

	tests := []struct {
		name  string
		input PatchProfileInput
	}{
		{name: "empty patch"},
		{name: "blank name", input: PatchProfileInput{Name: stringPointer("  ")}},
		{name: "name too long", input: PatchProfileInput{Name: stringPointer(longName())}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.PatchBasicProfile(context.Background(), "user-id", tt.input)
			if !errors.Is(err, ErrInvalidProfilePatch) {
				t.Fatalf("expected ErrInvalidProfilePatch, got %v", err)
			}
		})
	}
}

func TestPatchBasicProfileTrimsValuesAndAllowsClearingPicture(t *testing.T) {
	repo := &profilePatchRepository{response: &User{Name: "Updated"}}
	service := NewService(repo, nil)

	_, err := service.PatchBasicProfile(context.Background(), "user-id", PatchProfileInput{
		Name:    stringPointer("  Updated  "),
		Picture: stringPointer("  "),
	})
	if err != nil {
		t.Fatalf("PatchBasicProfile returned error: %v", err)
	}
	if !repo.called {
		t.Fatal("expected repository to be called")
	}
	if repo.input.Name == nil || *repo.input.Name != "Updated" {
		t.Errorf("expected trimmed name, got %v", repo.input.Name)
	}
	if repo.input.Picture == nil || *repo.input.Picture != "" {
		t.Errorf("expected empty picture to clear it, got %v", repo.input.Picture)
	}
}

func stringPointer(value string) *string {
	return &value
}

func longName() string {
	return strings.Repeat("x", 256)
}
