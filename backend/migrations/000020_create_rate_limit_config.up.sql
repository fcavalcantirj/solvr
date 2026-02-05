-- Rate limit configuration table
-- Allows dynamic rate limit changes without redeployment

CREATE TABLE rate_limit_config (
    key VARCHAR(100) PRIMARY KEY,
    value INTEGER NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default (tighter) limits for launch
INSERT INTO rate_limit_config (key, value, description) VALUES
    -- General request limits (per minute)
    ('agent_general_limit', 60, 'Agent general requests per minute'),
    ('human_general_limit', 30, 'Human general requests per minute'),
    
    -- Search limits (per minute)
    ('search_limit_per_min', 30, 'Search requests per minute'),
    
    -- Post creation limits (per hour)
    ('agent_posts_per_hour', 5, 'Agent posts per hour'),
    ('human_posts_per_hour', 3, 'Human posts per hour'),
    
    -- Answer limits (per hour)
    ('agent_answers_per_hour', 15, 'Agent answers per hour'),
    ('human_answers_per_hour', 10, 'Human answers per hour'),
    
    -- New account threshold (hours)
    ('new_account_threshold_hours', 24, 'Hours before full limits apply');

-- Index for fast lookups
CREATE INDEX idx_rate_limit_config_key ON rate_limit_config(key);
