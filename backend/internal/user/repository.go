package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) UpsertByGoogleSub(ctx context.Context, sub, email, name, picture string) (*User, error) {
	query := `
		INSERT INTO users (google_sub, email, name, picture, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (google_sub) DO UPDATE SET
			email = EXCLUDED.email,
			updated_at = NOW()
		RETURNING id, google_sub, email, name, picture, created_at, updated_at
	`

	var u User
	err := r.db.QueryRowContext(ctx, query, sub, email, name, picture).Scan(
		&u.ID,
		&u.GoogleSub,
		&u.Email,
		&u.Name,
		&u.Picture,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, google_sub, email, name, picture, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var u User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.GoogleSub,
		&u.Email,
		&u.Name,
		&u.Picture,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *PostgresRepository) PatchBasicProfile(ctx context.Context, userID string, in PatchProfileInput) (*User, error) {
	query := `
		UPDATE users
		SET name = COALESCE($2, name),
			picture = COALESCE($3, picture),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, google_sub, email, name, picture, created_at, updated_at
	`

	var u User
	err := r.db.QueryRowContext(ctx, query, userID, in.Name, in.Picture).Scan(
		&u.ID,
		&u.GoogleSub,
		&u.Email,
		&u.Name,
		&u.Picture,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *PostgresRepository) UpsertProfile(ctx context.Context, userID string, in UpsertProfileInput, embedding []float32) (*AcademicProfile, error) {
	query := `
		INSERT INTO user_profiles (user_id, university, major, target_subjects, study_mode, bio, embedding, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			university = EXCLUDED.university,
			major = EXCLUDED.major,
			target_subjects = EXCLUDED.target_subjects,
			study_mode = EXCLUDED.study_mode,
			bio = EXCLUDED.bio,
			embedding = EXCLUDED.embedding,
			updated_at = NOW()
		RETURNING user_id, university, major, target_subjects, study_mode, bio, updated_at
	`

	m := pgtype.NewMap()
	var p AcademicProfile
	var subjects []string

	err := r.db.QueryRowContext(
		ctx,
		query,
		userID,
		in.University,
		in.Major,
		in.TargetSubjects, // Atau cukup kirim slice langsung pada pgx/v5 stdlib
		in.StudyMode,
		in.Bio,
		pgvector.NewVector(embedding),
	).Scan(
		&p.UserID,
		&p.University,
		&p.Major,
		m.SQLScanner(&subjects),
		&p.StudyMode,
		&p.Bio,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	p.TargetSubjects = subjects
	return &p, nil
}

func (r *PostgresRepository) FindUserWithProfileByID(ctx context.Context, id string) (*UserWithProfile, error) {
	query := `
		SELECT
			u.id, u.google_sub, u.email, u.name, u.picture, u.created_at, u.updated_at,
			p.university, p.major, p.target_subjects, p.study_mode, p.bio, p.updated_at
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1
	`

	m := pgtype.NewMap()
	var res UserWithProfile
	var uni, major, mode, bio sql.NullString
	var subjects []string
	var profUpdatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&res.ID, &res.GoogleSub, &res.Email, &res.Name, &res.Picture, &res.CreatedAt, &res.UpdatedAt,
		&uni, &major, m.SQLScanner(&subjects), &mode, &bio, &profUpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if uni.Valid {
		res.Profile = &AcademicProfile{
			UserID:         res.ID,
			University:     uni.String,
			Major:          major.String,
			TargetSubjects: subjects,
			StudyMode:      mode.String,
			Bio:            bio.String,
			UpdatedAt:      profUpdatedAt.Time,
		}
	}

	return &res, nil
}
