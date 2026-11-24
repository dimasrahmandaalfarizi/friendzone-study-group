package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// GetWorkspaceAndPartner memvalidasi keanggotaan user sekaligus mengembalikan ID partner belajarnya
func (r *PostgresRepository) GetWorkspaceAndPartner(ctx context.Context, workspaceID, userID string) (*WorkspaceDetail, error) {
	query := `
		SELECT
			w.id,
			w.match_id,
			w.name,
			u.id AS partner_id,
			u.name AS partner_name,
			u.picture,
			w.created_at,
			(m.user1_id = $2 OR m.user2_id = $2) AS is_member
		FROM workspaces w
		JOIN matches m ON m.id = w.match_id
		JOIN users u ON u.id = CASE WHEN m.user1_id = $2 THEN m.user2_id ELSE m.user1_id END
		WHERE w.id = $1
	`

	var ws WorkspaceDetail
	var isMember bool

	err := r.db.QueryRowContext(ctx, query, workspaceID, userID).Scan(
		&ws.ID,
		&ws.MatchID,
		&ws.Name,
		&ws.PartnerID,
		&ws.PartnerName,
		&ws.PartnerPic,
		&ws.CreatedAt,
		&isMember,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrForbiddenAccess
	}

	return &ws, nil
}

// --- COLLABORATIVE NOTES ---

func (r *PostgresRepository) ListNotes(ctx context.Context, workspaceID string) ([]Note, error) {
	query := `
		SELECT id, workspace_id, title, content_json, updated_by, created_at, updated_at
		FROM workspace_notes
		WHERE workspace_id = $1
		ORDER BY updated_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]Note, 0)
	for rows.Next() {
		var n Note
		var rawContent []byte
		var updatedBy sql.NullString

		if err := rows.Scan(&n.ID, &n.WorkspaceID, &n.Title, &rawContent, &updatedBy, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.ContentJSON = json.RawMessage(rawContent)
		if updatedBy.Valid {
			n.UpdatedBy = &updatedBy.String
		}
		notes = append(notes, n)
	}

	return notes, rows.Err()
}

func (r *PostgresRepository) CreateNote(ctx context.Context, workspaceID, userID string, in UpsertNoteInput) (*Note, error) {
	query := `
		INSERT INTO workspace_notes (workspace_id, title, content_json, updated_by)
		VALUES ($1, $2, $3::jsonb, $4)
		RETURNING id, workspace_id, title, content_json, updated_by, created_at, updated_at
	`
	var n Note
	var rawContent []byte
	var updatedBy sql.NullString

	err := r.db.QueryRowContext(ctx, query, workspaceID, in.Title, string(in.ContentJSON), userID).Scan(
		&n.ID, &n.WorkspaceID, &n.Title, &rawContent, &updatedBy, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	n.ContentJSON = json.RawMessage(rawContent)
	if updatedBy.Valid {
		n.UpdatedBy = &updatedBy.String
	}
	return &n, nil
}

func (r *PostgresRepository) UpdateNote(ctx context.Context, workspaceID, noteID, userID string, in UpsertNoteInput) (*Note, error) {
	query := `
		UPDATE workspace_notes
		SET title = $1,
		    content_json = $2::jsonb,
		    updated_by = $3,
		    updated_at = NOW()
		WHERE id = $4 AND workspace_id = $5
		RETURNING id, workspace_id, title, content_json, updated_by, created_at, updated_at
	`
	var n Note
	var rawContent []byte
	var updatedBy sql.NullString

	err := r.db.QueryRowContext(ctx, query, in.Title, string(in.ContentJSON), userID, noteID, workspaceID).Scan(
		&n.ID, &n.WorkspaceID, &n.Title, &rawContent, &updatedBy, &n.CreatedAt, &n.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoteNotFound
	}
	if err != nil {
		return nil, err
	}

	n.ContentJSON = json.RawMessage(rawContent)
	if updatedBy.Valid {
		n.UpdatedBy = &updatedBy.String
	}
	return &n, nil
}

// --- STUDY SESSIONS ---

func (r *PostgresRepository) ListSessions(ctx context.Context, workspaceID string) ([]StudySession, error) {
	query := `
		SELECT id, workspace_id, created_by, topic, scheduled_at, meeting_link, status, created_at, updated_at
		FROM study_sessions
		WHERE workspace_id = $1
		ORDER BY scheduled_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]StudySession, 0)
	for rows.Next() {
		var s StudySession
		if err := rows.Scan(
			&s.ID, &s.WorkspaceID, &s.CreatedBy, &s.Topic, &s.ScheduledAt, &s.MeetingLink, &s.Status, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

func (r *PostgresRepository) CreateSession(ctx context.Context, workspaceID, userID string, in CreateSessionInput) (*StudySession, error) {
	query := `
		INSERT INTO study_sessions (workspace_id, created_by, topic, scheduled_at, meeting_link)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, workspace_id, created_by, topic, scheduled_at, meeting_link, status, created_at, updated_at
	`
	var s StudySession
	err := r.db.QueryRowContext(ctx, query, workspaceID, userID, in.Topic, in.ScheduledAt, in.MeetingLink).Scan(
		&s.ID, &s.WorkspaceID, &s.CreatedBy, &s.Topic, &s.ScheduledAt, &s.MeetingLink, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PostgresRepository) UpdateSessionStatus(ctx context.Context, workspaceID, sessionID, status string) (*StudySession, error) {
	query := `
		UPDATE study_sessions
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND workspace_id = $3
		RETURNING id, workspace_id, created_by, topic, scheduled_at, meeting_link, status, created_at, updated_at
	`
	var s StudySession
	err := r.db.QueryRowContext(ctx, query, status, sessionID, workspaceID).Scan(
		&s.ID, &s.WorkspaceID, &s.CreatedBy, &s.Topic, &s.ScheduledAt, &s.MeetingLink, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// --- PEER EVALUATIONS ---

func (r *PostgresRepository) CreateEvaluation(
	ctx context.Context,
	workspaceID, sessionID, reviewerID, revieweeID string,
	in CreateEvaluationInput,
) (*Evaluation, error) {
	// Pastikan session benar-benar milik workspace ini
	var sessionExists bool
	checkSession := `SELECT EXISTS(SELECT 1 FROM study_sessions WHERE id = $1 AND workspace_id = $2)`
	if err := r.db.QueryRowContext(ctx, checkSession, sessionID, workspaceID).Scan(&sessionExists); err != nil {
		return nil, err
	}
	if !sessionExists {
		return nil, ErrSessionNotFound
	}

	var alreadySubmitted bool
	checkDup := `SELECT EXISTS(SELECT 1 FROM session_evaluations WHERE session_id = $1 AND reviewer_id = $2)`
	if err := r.db.QueryRowContext(ctx, checkDup, sessionID, reviewerID).Scan(&alreadySubmitted); err != nil {
		return nil, err
	}
	if alreadySubmitted {
		return nil, ErrAlreadyEvaluated
	}

	query := `
		INSERT INTO session_evaluations (session_id, reviewer_id, reviewee_id, rating, reflection_note)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, session_id, reviewer_id, reviewee_id, rating, reflection_note, created_at
	`
	var ev Evaluation
	err := r.db.QueryRowContext(ctx, query, sessionID, reviewerID, revieweeID, in.Rating, in.ReflectionNote).Scan(
		&ev.ID, &ev.SessionID, &ev.ReviewerID, &ev.RevieweeID, &ev.Rating, &ev.ReflectionNote, &ev.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}