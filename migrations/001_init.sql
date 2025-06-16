-- Initial database schema for WebTunnel

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    uuid VARCHAR(36) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Sessions table (for web sessions, not terminal sessions)
CREATE TABLE IF NOT EXISTS user_sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Terminal sessions table
CREATE TABLE IF NOT EXISTS terminal_sessions (
    id SERIAL PRIMARY KEY,
    uuid VARCHAR(36) UNIQUE NOT NULL,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    command TEXT NOT NULL,
    working_directory VARCHAR(500),
    status VARCHAR(50) DEFAULT 'running',
    pid INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP
);

-- Session sharing table
CREATE TABLE IF NOT EXISTS session_shares (
    id SERIAL PRIMARY KEY,
    session_id INTEGER REFERENCES terminal_sessions(id) ON DELETE CASCADE,
    share_token VARCHAR(255) UNIQUE NOT NULL,
    created_by INTEGER REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audit log table
CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(100),
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_uuid ON users(uuid);
CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(session_token);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_terminal_sessions_uuid ON terminal_sessions(uuid);
CREATE INDEX IF NOT EXISTS idx_terminal_sessions_user_id ON terminal_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_terminal_sessions_status ON terminal_sessions(status);
CREATE INDEX IF NOT EXISTS idx_session_shares_token ON session_shares(share_token);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

-- Insert a demo user for testing
INSERT INTO users (uuid, email, username, password_hash, role) 
VALUES (
    'demo-user-uuid-12345678', 
    'demo@example.com', 
    'demo', 
    '$2a$10$dummy.hash.for.demo.purposes', 
    'admin'
) ON CONFLICT (email) DO NOTHING;