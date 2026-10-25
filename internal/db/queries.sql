-- name: GetChallengeByTopic :one
SELECT * FROM challenges
WHERE chat_id = ? AND thread_id = ? AND month = ?;

-- name: GetChallenge :one
SELECT * FROM challenges
WHERE id = ?;

-- name: ListChallengesByMonth :many
SELECT * FROM challenges
WHERE month = ?;

-- name: CreateChallenge :one
INSERT INTO challenges (chat_id, thread_id, month, stats_message_id, start_day)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: CreateProof :exec
INSERT INTO proofs (challenge_id, message_id, user_id, user_name, day)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: FindProof :one
SELECT p.* FROM proofs p
JOIN challenges c ON c.id = p.challenge_id
WHERE c.chat_id = ? AND p.message_id = ?;

-- name: AddReaction :exec
INSERT INTO reactions (challenge_id, message_id, user_id)
VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: CountReactions :one
SELECT COUNT(*) FROM reactions
WHERE challenge_id = ? AND message_id = ?;

-- name: RemoveReaction :exec
DELETE FROM reactions
WHERE challenge_id = ? AND message_id = ? AND user_id = ?;

-- name: ListProofStats :many
SELECT p.user_id, p.user_name, p.day, COUNT(r.user_id) AS reactors
FROM proofs p
LEFT JOIN reactions r ON r.challenge_id = p.challenge_id AND r.message_id = p.message_id
WHERE p.challenge_id = ?
GROUP BY p.challenge_id, p.message_id
ORDER BY p.message_id;
