-- Initial schema for the review storage service.

CREATE TABLE IF NOT EXISTS repos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_owner TEXT NOT NULL,
    github_repo  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(github_owner, github_repo)
);

CREATE TABLE IF NOT EXISTS participants (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS reviews (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id    UUID NOT NULL REFERENCES repos(id),
    title      TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'open',
    documents  JSONB NOT NULL DEFAULT '[]',
    source_ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reviews_repo ON reviews(repo_id);

CREATE TABLE IF NOT EXISTS review_reviewers (
    review_id           UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    participant_id      TEXT NOT NULL REFERENCES participants(id),
    status              TEXT NOT NULL DEFAULT 'pending',
    approved_at         TIMESTAMPTZ,
    approval_source_ref TEXT,
    PRIMARY KEY (review_id, participant_id)
);

CREATE TABLE IF NOT EXISTS threads (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id    UUID NOT NULL REFERENCES repos(id),
    review_id  UUID REFERENCES reviews(id),
    document   TEXT NOT NULL,
    anchor     JSONB NOT NULL,
    status     TEXT NOT NULL DEFAULT 'open',
    version    INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_threads_repo_document ON threads(repo_id, document);
CREATE INDEX IF NOT EXISTS idx_threads_repo_status ON threads(repo_id, status);

CREATE TABLE IF NOT EXISTS comments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id  UUID NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    author     TEXT NOT NULL REFERENCES participants(id),
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_comments_thread ON comments(thread_id);
