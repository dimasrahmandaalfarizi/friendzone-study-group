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
