CREATE TABLE IF NOT EXISTS etfs (
    id VARCHAR(255) PRIMARY KEY,
    data JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP
    );

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255),
    password VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP
);

-- Add a sample user to the 'users' table
INSERT INTO users (username, password) VALUES
    ('admin', '8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918');

-- Create a trigger function to update the updated_at column
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create a trigger to call the set_updated_at function before update
CREATE TRIGGER set_updated_at_trigger
    BEFORE UPDATE ON etfs
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
