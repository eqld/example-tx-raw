-- init.sql
-- This script is executed when the PostgreSQL container starts.

-- Drop the table if it exists to ensure a clean state for each run
DROP TABLE IF EXISTS items;

-- Create a simple table for demonstration
CREATE TABLE items (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    data TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

GRANT ALL PRIVILEGES ON TABLE items TO exampleuser;
GRANT USAGE, SELECT ON SEQUENCE items_id_seq TO exampleuser;
