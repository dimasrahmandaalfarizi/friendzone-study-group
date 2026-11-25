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

type academicProfileRepository struct {
	embedding []float32
	input     UpsertProfileInput
	called    bool
}

func (r *academicProfileRepository) UpsertByGoogleSub(context.Context, string, string, string, string) (*User, error) {
	return nil, nil
}

func (r *academicProfileRepository) FindByID(context.Context, string) (*User, error) { return nil, nil }
func (r *academicProfileRepository) PatchBasicProfile(context.Context, string, PatchProfileInput) (*User, error) {
	return nil, nil
}
func (r *academicProfileRepository) UpsertProfile(_ context.Context, _ string, in UpsertProfileInput, embedding []float32) (*AcademicProfile, error) {
	r.input = in
	r.embedding = embedding
	r.called = true
	return &AcademicProfile{University: in.University, Major: in.Major}, nil
}
func (r *academicProfileRepository) FindUserWithProfileByID(context.Context, string) (*UserWithProfile, error) {
	return nil, nil
}

type mockEmbedder struct {
	vector []float32
	err    error
	text   string
}

func (e *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.text = text
	return e.vector, e.err
}

func TestUpdateAcademicProfileValidationAndEmbedding(t *testing.T) {
	tests := []struct {
		name  string
		input UpsertProfileInput
	}{
		{name: "blank university", input: UpsertProfileInput{University: " ", Major: "CS", TargetSubjects: []string{"Go"}, StudyMode: "hybrid"}},
		{name: "blank major", input: UpsertProfileInput{University: "ITB", Major: " ", TargetSubjects: []string{"Go"}, StudyMode: "hybrid"}},
		{name: "no subjects", input: UpsertProfileInput{University: "ITB", Major: "CS", StudyMode: "hybrid"}},
		{name: "invalid mode", input: UpsertProfileInput{University: "ITB", Major: "CS", TargetSubjects: []string{"Go"}, StudyMode: "offline"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &academicProfileRepository{}
			service := NewService(repo, &mockEmbedder{})
			if _, err := service.UpdateAcademicProfile(context.Background(), "u1", tt.input); err == nil {
				t.Fatal("expected validation error")
			}
			if repo.called {
				t.Fatal("repository must not be called for invalid input")
			}
		})
	}
}

func TestUpdateAcademicProfileEmbeds768DimensionsAndNormalizes(t *testing.T) {
	repo := &academicProfileRepository{}
	embedder := &mockEmbedder{vector: make([]float32, 768)}
	service := NewService(repo, embedder)

	in := UpsertProfileInput{
		University:     "  ITB  ",
		Major:          "  CS ",
		TargetSubjects: []string{"Go", "Algorithms"},
		StudyMode:      "  HYBRID ",
		Bio:            "  wants group study ",
	}
	if _, err := service.UpdateAcademicProfile(context.Background(), "u1", in); err != nil {
		t.Fatalf("UpdateAcademicProfile returned error: %v", err)
	}
	if repo.input.StudyMode != "hybrid" || repo.input.University != "ITB" || repo.input.Major != "CS" {
		t.Fatalf("input was not normalized: %+v", repo.input)
	}
	if len(repo.embedding) != 768 {
		t.Fatalf("expected 768 dimensions, got %d", len(repo.embedding))
	}
	for _, fragment := range []string{"University: ITB", "Major: CS", "Go, Algorithms", "hybrid", "wants group study"} {
		if !strings.Contains(embedder.text, fragment) {
			t.Errorf("semantic text %q missing %q", embedder.text, fragment)
		}
	}
}

func TestUpdateAcademicProfileRejectsEmbedderFailureAndWrongDimensions(t *testing.T) {
	tests := []struct {
		name     string
		embedder *mockEmbedder
	}{
		{name: "embedder error", embedder: &mockEmbedder{err: errors.New("embedding unavailable")}},
		{name: "wrong dimensions", embedder: &mockEmbedder{vector: make([]float32, 767)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &academicProfileRepository{}
			service := NewService(repo, tt.embedder)
			in := UpsertProfileInput{University: "ITB", Major: "CS", TargetSubjects: []string{"Go"}, StudyMode: "hybrid"}
			if _, err := service.UpdateAcademicProfile(context.Background(), "u1", in); err == nil {
				t.Fatal("expected error")
			}
			if repo.called {
				t.Fatal("UpsertProfile must only run after a valid embedding")
			}
		})
	}
}
