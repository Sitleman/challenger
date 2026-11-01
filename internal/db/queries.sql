-- name: GetChallengeByTopic :one
SELECT * FROM challenges
WHERE chat_id = ? AND thread_id = ? AND month = ?;

-- name: GetLatestChallengeByTopic :one
SELECT * FROM challenges
WHERE chat_id = ? AND thread_id = ? AND month <= ?
ORDER BY month DESC
LIMIT 1;

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

-- name: StopChallenge :one
UPDATE challenges SET stopped_at = ?
WHERE id = ? AND stopped_at = 0
RETURNING *;

-- name: SetSummaryText :exec
UPDATE challenges SET summary_text = ?
WHERE id = ?;

-- name: SetSummaryProgress :exec
UPDATE challenges SET summary_message_id = ?, summary_parts_sent = ?
WHERE id = ?;

-- name: CreateProof :exec
INSERT INTO proofs (challenge_id, message_id, user_id, user_name, day)
SELECT sqlc.arg(challenge_id), sqlc.arg(message_id), sqlc.arg(user_id), sqlc.arg(user_name), sqlc.arg(day)
WHERE EXISTS (SELECT 1 FROM challenges WHERE id = sqlc.arg(challenge_id) AND stopped_at = 0)
ON CONFLICT DO NOTHING;

-- name: FindProof :one
SELECT p.* FROM proofs p
JOIN challenges c ON c.id = p.challenge_id
WHERE c.chat_id = ? AND p.message_id = ? AND c.stopped_at = 0;

-- name: AddReaction :exec
INSERT INTO reactions (challenge_id, message_id, user_id, user_name)
SELECT sqlc.arg(challenge_id), sqlc.arg(message_id), sqlc.arg(user_id), sqlc.arg(user_name)
WHERE EXISTS (SELECT 1 FROM challenges WHERE id = sqlc.arg(challenge_id) AND stopped_at = 0)
ON CONFLICT (challenge_id, message_id, user_id) DO UPDATE SET user_name = excluded.user_name;

-- name: CountReactions :one
SELECT COUNT(*) FROM reactions
WHERE challenge_id = ? AND message_id = ?;

-- name: RemoveReaction :exec
DELETE FROM reactions
WHERE challenge_id = sqlc.arg(challenge_id) AND message_id = sqlc.arg(message_id) AND user_id = sqlc.arg(user_id)
AND EXISTS (SELECT 1 FROM challenges WHERE id = sqlc.arg(challenge_id) AND stopped_at = 0);

-- name: ListProofStats :many
SELECT p.message_id, p.user_id, p.user_name, p.day, COUNT(r.user_id) AS reactors
FROM proofs p
LEFT JOIN reactions r ON r.challenge_id = p.challenge_id AND r.message_id = p.message_id
WHERE p.challenge_id = ?
GROUP BY p.challenge_id, p.message_id
ORDER BY p.message_id;

-- name: ListSupportStats :many
SELECT r.user_id,
    CAST(COALESCE(
        (SELECT r2.user_name FROM reactions r2
         WHERE r2.challenge_id = r.challenge_id AND r2.user_id = r.user_id AND r2.user_name != ''
         ORDER BY r2.message_id DESC LIMIT 1),
        (SELECT p2.user_name FROM proofs p2
         WHERE p2.challenge_id = r.challenge_id AND p2.user_id = r.user_id
         ORDER BY p2.message_id DESC LIMIT 1),
        ''
    ) AS TEXT) AS user_name,
    COUNT(DISTINCT p.user_id) AS recipients
FROM reactions r
JOIN proofs p ON p.challenge_id = r.challenge_id AND p.message_id = r.message_id
JOIN challenges c ON c.id = r.challenge_id
WHERE r.challenge_id = sqlc.arg(challenge_id) AND r.user_id != p.user_id
AND p.day >= c.start_day AND p.day <= sqlc.arg(end_day)
GROUP BY r.challenge_id, r.user_id
ORDER BY recipients DESC, r.user_id;
