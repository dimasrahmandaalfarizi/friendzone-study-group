package match

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetUserEmbeddingAndCampus(ctx context.Context, userID string) (pgvector.Vector, string, error) {
	var vec pgvector.Vector
	var university string
	var isNull bool

	query := `
		SELECT university, embedding IS NULL, COALESCE(embedding, array_fill(0, ARRAY[768])::vector)
		FROM user_profiles
		WHERE user_id = $1
	`
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&university, &isNull, &vec)
	if errors.Is(err, sql.ErrNoRows) || isNull {
		return pgvector.Vector{}, "", ErrProfileIncomplete
	}
	if err != nil {
		return pgvector.Vector{}, "", err
	}

	return vec, university, nil
}

func (r *PostgresRepository) FetchDiscoveryDeck(
	ctx context.Context,
	currentUserID string,
	userVec pgvector.Vector,
	universityFilter string,
	limit int,
) ([]PeerCard, error) {
	query := `
		SELECT
			u.id,
			u.name,
			u.picture,
			p.university,
			p.major,
			p.target_subjects,
			p.study_mode,
			p.bio,
			GREATEST(0, ROUND(((1 - (p.embedding <=> $1)) * 100)::numeric, 1)) AS compatibility_pct
		FROM user_profiles p
		JOIN users u ON u.id = p.user_id
		WHERE p.user_id != $2
		  AND p.embedding IS NOT NULL
		  AND NOT EXISTS (
			  SELECT 1 FROM swipes s
			  WHERE s.swiper_id = $2 AND s.target_id = p.user_id
		  )
	`

	args := []any{userVec, currentUserID}
	argIdx := 3

	if universityFilter != "" {
		query += fmt.Sprintf(" AND LOWER(p.university) = LOWER($%d)", argIdx)
		args = append(args, universityFilter)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY p.embedding <=> $1 ASC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := pgtype.NewMap()
	cards := make([]PeerCard, 0)

	for rows.Next() {
		var card PeerCard
		var subjects []string
		if err := rows.Scan(
			&card.UserID,
			&card.Name,
			&card.Picture,
			&card.University,
			&card.Major,
			m.SQLScanner(&subjects),
			&card.StudyMode,
			&card.Bio,
			&card.CompatibilityPct,
		); err != nil {
			return nil, err
		}
		card.TargetSubjects = subjects
		cards = append(cards, card)
	}

	return cards, rows.Err()
}

func (r *PostgresRepository) RecordSwipe(ctx context.Context, swiperID, targetID, direction string) (*SwipeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Simpan aksi swipe
	upsertSwipe := `
		INSERT INTO swipes (swiper_id, target_id, direction, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (swiper_id, target_id) DO UPDATE SET
			direction = EXCLUDED.direction,
			created_at = NOW()
	`
	if _, err := tx.ExecContext(ctx, upsertSwipe, swiperID, targetID, direction); err != nil {
		return nil, err
	}

	// Jika swipe kiri, selesai tanpa cek mutual match
	if direction == "left" {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &SwipeResult{Matched: false}, nil
	}

	// 2. Cek apakah target sudah pernah swipe kanan ke swiper
	var isMutual bool
	checkMutual := `
		SELECT EXISTS(
			SELECT 1 FROM swipes
			WHERE swiper_id = $1 AND target_id = $2 AND direction = 'right'
		)
	`
	if err := tx.QueryRowContext(ctx, checkMutual, targetID, swiperID).Scan(&isMutual); err != nil {
		return nil, err
	}

	if !isMutual {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &SwipeResult{Matched: false}, nil
	}

	// 3. Mutual match! Pastikan urutan user1_id < user2_id
	u1, u2 := swiperID, targetID
	if u1 > u2 {
		u1, u2 = u2, u1
	}

	var matchID string
	insertMatch := `
		INSERT INTO matches (user1_id, user2_id)
		VALUES ($1, $2)
		ON CONFLICT (user1_id, user2_id) DO UPDATE SET user1_id = EXCLUDED.user1_id
		RETURNING id
	`
	if err := tx.QueryRowContext(ctx, insertMatch, u1, u2).Scan(&matchID); err != nil {
		return nil, err
	}

	var workspaceID string
	insertWorkspace := `
		INSERT INTO workspaces (match_id, name)
		VALUES ($1, 'Study Group Workspace')
		ON CONFLICT (match_id) DO UPDATE SET match_id = EXCLUDED.match_id
		RETURNING id
	`
	if err := tx.QueryRowContext(ctx, insertWorkspace, matchID).Scan(&workspaceID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &SwipeResult{
		Matched:     true,
		MatchID:     &matchID,
		WorkspaceID: &workspaceID,
	}, nil
}

func (r *PostgresRepository) ListMutualMatches(ctx context.Context, userID string) ([]MutualMatch, error) {
	query := `
		SELECT
			m.id AS match_id,
			w.id AS workspace_id,
			u.id AS partner_id,
			u.name AS partner_name,
			u.picture,
			COALESCE(p.university, ''),
			COALESCE(p.major, ''),
			m.created_at
		FROM matches m
		JOIN workspaces w ON w.match_id = m.id
		JOIN users u ON u.id = CASE WHEN m.user1_id = $1 THEN m.user2_id ELSE m.user1_id END
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE m.user1_id = $1 OR m.user2_id = $1
		ORDER BY m.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]MutualMatch, 0)
	for rows.Next() {
		var mm MutualMatch
		if err := rows.Scan(
			&mm.MatchID,
			&mm.WorkspaceID,
			&mm.PartnerID,
			&mm.PartnerName,
			&mm.Picture,
			&mm.University,
			&mm.Major,
			&mm.MatchedAt,
		); err != nil {
			return nil, err
		}
		matches = append(matches, mm)
	}

	return matches, rows.Err()
}