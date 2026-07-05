-- Database initialization script for FreeCompute
-- This runs automatically when the PostgreSQL container starts for the first time.

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(254) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    verified BOOLEAN DEFAULT FALSE,
    role VARCHAR(20) DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    credits INTEGER DEFAULT 0 CHECK (credits >= 0),
    verification_token UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Hosts table
CREATE TABLE IF NOT EXISTS hosts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    region VARCHAR(50) NOT NULL,
    cpu_cores INTEGER NOT NULL CHECK (cpu_cores > 0),
    ram_gb INTEGER NOT NULL CHECK (ram_gb > 0),
    gpu_vram_gb INTEGER DEFAULT 0,
    online BOOLEAN DEFAULT FALSE,
    approved BOOLEAN DEFAULT FALSE,
    last_heartbeat TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- VMs table
CREATE TABLE IF NOT EXISTS vms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    host_id UUID REFERENCES hosts(id),
    name VARCHAR(64) NOT NULL,
    state VARCHAR(20) DEFAULT 'stopped' CHECK (state IN ('starting', 'running', 'paused', 'stopped')),
    cpu_cores INTEGER NOT NULL CHECK (cpu_cores BETWEEN 1 AND 16),
    ram_gb INTEGER NOT NULL CHECK (ram_gb BETWEEN 1 AND 64),
    storage_gb INTEGER NOT NULL CHECK (storage_gb BETWEEN 10 AND 500),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Queue table
CREATE TABLE IF NOT EXISTS queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    estimated_wait_seconds INTEGER DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('purchase', 'spend', 'reward', 'refund')),
    amount INTEGER NOT NULL,
    description VARCHAR(500),
    idempotency_key UUID UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Files table
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_name VARCHAR(255) NOT NULL,
    storage_path TEXT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Sessions table (for token management)
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN DEFAULT FALSE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_vms_user_id ON vms(user_id);
CREATE INDEX IF NOT EXISTS idx_vms_host_id ON vms(host_id);
CREATE INDEX IF NOT EXISTS idx_queue_user_id ON queue(user_id);
CREATE INDEX IF NOT EXISTS idx_queue_position ON queue(position);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_idempotency ON transactions(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_hosts_online ON hosts(online) WHERE online = true;

-- Set statement timeout to prevent long-running query DoS (10 seconds)
ALTER DATABASE freecompute SET statement_timeout = '10s';
