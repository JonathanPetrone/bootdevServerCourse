-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";  

CREATE TABLE chirps (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),  
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    body TEXT NOT NULL,
    user_id uuid REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE chirps;