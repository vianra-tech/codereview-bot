-- 001_initial_schema.up.sql
-- Initial database schema for CodeReview.ai

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Organizations (GitHub orgs / GitLab groups / standalone)
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    provider VARCHAR(50) NOT NULL, -- github, gitlab, standalone
    provider_id VARCHAR(100), -- GitHub org ID
    settings JSONB DEFAULT '{}', -- billing, preferences, policies
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Repositories
CREATE TABLE repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    provider_repo_id VARCHAR(100) NOT NULL,
    full_name VARCHAR(500) NOT NULL, -- org/repo
    default_branch VARCHAR(255) DEFAULT 'main',
    language VARCHAR(100),
    is_private BOOLEAN DEFAULT true,
    settings JSONB DEFAULT '{}', -- rule packs, severity overrides, notifications
    installation_id VARCHAR(100), -- GitHub App installation ID
    webhook_secret_encrypted BYTEA, -- encrypted webhook secret
    last_analyzed_at TIMESTAMPTZ,
    health_score SMALLINT, -- 0-100
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (provider, provider_repo_id)
);

-- Analysis Runs (one per push/PR event)
CREATE TABLE analysis_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id UUID REFERENCES repositories(id) ON DELETE CASCADE,
    trigger_event VARCHAR(50) NOT NULL, -- push, pull_request, merge, manual
    trigger_ref VARCHAR(500), -- refs/heads/main, refs/pull/123/merge
    commit_sha CHAR(40) NOT NULL,
    pr_number INTEGER, -- nullable for push events
    status VARCHAR(20) DEFAULT 'pending', -- pending, running, completed, failed
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER,
    summary JSONB, -- aggregated counts by severity/type
    sarif_location TEXT, -- S3 path
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Findings (individual issues)
CREATE TABLE findings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID REFERENCES analysis_runs(id) ON DELETE CASCADE,
    tool VARCHAR(50) NOT NULL, -- static, security, semantic, semantic:fix
    rule_id VARCHAR(200) NOT NULL,
    fingerprint VARCHAR(64) NOT NULL, -- SHA256 for deduplication
    type VARCHAR(50), -- security, logic, performance, style, architecture
    severity VARCHAR(20) NOT NULL, -- critical, high, medium, low, info
    title VARCHAR(500) NOT NULL,
    description TEXT,
    file_path VARCHAR(1000) NOT NULL,
    start_line INTEGER,
    end_line INTEGER,
    start_column INTEGER,
    end_column INTEGER,
    code_snippet TEXT,
    suggestion TEXT,
    fix_diff TEXT, -- unified diff if auto-fix generated
    confidence REAL, -- 0.0-1.0 for LLM findings
    metadata JSONB DEFAULT '{}', -- tool-specific data
    status VARCHAR(20) DEFAULT 'open', -- open, fixed, dismissed, suppressed
    dismissed_by UUID, -- user ID
    dismissed_at TIMESTAMPTZ,
    dismissal_reason TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_findings_run_id ON findings(run_id);
CREATE INDEX idx_findings_fingerprint ON findings(fingerprint);
CREATE INDEX idx_findings_file_severity ON findings(file_path, severity);
CREATE INDEX idx_findings_status ON findings(status) WHERE status = 'open';
CREATE INDEX idx_analysis_runs_repo_created ON analysis_runs(repo_id, created_at DESC);

-- Team Learning Patterns (suppressions, conventions)
CREATE TABLE learning_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    pattern_type VARCHAR(50) NOT NULL, -- suppression, convention, fix_preference
    rule_id VARCHAR(200),
    file_pattern VARCHAR(500), -- glob pattern
    context_embedding VECTOR(768), -- for semantic similarity
    pattern_data JSONB NOT NULL, -- the learned pattern
    confidence REAL DEFAULT 0.5,
    sample_count INTEGER DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Subscriptions & Billing
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    stripe_customer_id VARCHAR(100) UNIQUE,
    stripe_subscription_id VARCHAR(100) UNIQUE,
    plan VARCHAR(50) NOT NULL, -- free, pro, team, enterprise
    status VARCHAR(20) DEFAULT 'active', -- active, past_due, canceled, trialing
    seats INTEGER DEFAULT 1,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Audit Log (immutable)
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    user_id UUID, -- nullable for system actions
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    old_values JSONB,
    new_values JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_audit_org_created ON audit_logs(org_id, created_at DESC);

-- Row Level Security (RLS) policies
ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE repositories ENABLE ROW LEVEL SECURITY;
ALTER TABLE analysis_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE findings ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning_patterns ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

-- RLS Policies (service role bypasses all)
-- These policies ensure org-level isolation when using anon/user tokens