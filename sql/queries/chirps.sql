-- name: CreateChirp :one
INSERT INTO chirps(created_at, updated_at, body, user_id)
VALUES (
    NOW(),
    NOW(),
    @body,
    @user_id
)
RETURNING *;

-- name: GetAllChirps :many
SELECT * FROM chirps
ORDER BY created_at ASC;

-- name: GetOneChirp :one
SELECT * FROM chirps
WHERE id = $1;

-- name: DeleteOneChirp :exec
DELETE FROM chirps
WHERE id = $1;