CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  google_sub VARCHAR(255) UNIQUE NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  name VARCHAR(255) NOT NULL,
  picture TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_profiles (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  university VARCHAR(255) NOT NULL,
  major VARCHAR(255) NOT NULL,
  target_subjects TEXT[] NOT NULL DEFAULT '{}',
  study_mode VARCHAR(32) NOT NULL CHECK (study_mode IN ('online', 'in-person', 'hybrid')),
  bio TEXT NOT NULL DEFAULT '',
  embedding vector(768),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_profiles_embedding
ON user_profiles USING hnsw (embedding vector_cosine_ops);

-- 1. Mencatat semua swipe kiri & kanan
CREATE TABLE swipes (
  swiper_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  target_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  direction VARCHAR(10) NOT NULL CHECK (direction IN ('left', 'right')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (swiper_id, target_id),
  CONSTRAINT chk_no_self_swipe CHECK (swiper_id != target_id)
);

-- Index parsial khusus 'right' untuk cek mutual match dalam O(log n)
CREATE INDEX idx_swipes_target_right ON swipes(target_id, swiper_id) WHERE direction = 'right';

-- 2. Menyimpan pasangan yang saling swipe kanan (Mutual Match)
CREATE TABLE matches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user1_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  user2_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Menjamin urutan ID selalu konsisten sehingga pasangan (A,B) tidak pernah terduplikasi
  CONSTRAINT chk_user_order CHECK (user1_id < user2_id),
  CONSTRAINT uq_mutual_match UNIQUE (user1_id, user2_id)
);

CREATE INDEX idx_matches_user1 ON matches(user1_id);
CREATE INDEX idx_matches_user2 ON matches(user2_id);

-- 3. Shared Workspace otomatis terbentuk ketika terjadi mutual match
CREATE TABLE workspaces (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  match_id UUID UNIQUE NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 1. Catatan kolaboratif (menyimpan struktur JSON dari editor Tiptap)
CREATE TABLE IF NOT EXISTS workspace_notes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  title VARCHAR(255) NOT NULL DEFAULT 'Untitled Study Note',
  content_json JSONB NOT NULL DEFAULT '{"type":"doc","content":[]}'::jsonb,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_notes_ws ON workspace_notes(workspace_id);

-- 2. Penjadwalan sesi belajar
CREATE TABLE IF NOT EXISTS study_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  topic VARCHAR(255) NOT NULL,
  scheduled_at TIMESTAMPTZ NOT NULL,
  meeting_link TEXT NOT NULL DEFAULT '', -- buat simpan link meet/zoom
  status VARCHAR(20) NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'completed', 'cancelled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_study_sessions_ws ON study_sessions(workspace_id);

-- 3. Evaluasi akuntabilitas & rating peer pasca-sesi (mencegah free-riding)
CREATE TABLE IF NOT EXISTS session_evaluations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id UUID NOT NULL REFERENCES study_sessions(id) ON DELETE CASCADE,
  reviewer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reviewee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  rating INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  reflection_note TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_no_self_eval CHECK (reviewer_id != reviewee_id),
  CONSTRAINT uq_session_reviewer UNIQUE (session_id, reviewer_id)
);

CREATE INDEX IF NOT EXISTS idx_session_evaluations_reviewee ON session_evaluations(reviewee_id);
