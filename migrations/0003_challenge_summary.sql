-- +goose Up
ALTER TABLE challenges ADD COLUMN stopped_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE challenges ADD COLUMN summary_message_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE challenges ADD COLUMN summary_text TEXT NOT NULL DEFAULT '';
ALTER TABLE challenges ADD COLUMN summary_parts_sent INTEGER NOT NULL DEFAULT 0;
ALTER TABLE reactions ADD COLUMN user_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE reactions DROP COLUMN user_name;
ALTER TABLE challenges DROP COLUMN summary_parts_sent;
ALTER TABLE challenges DROP COLUMN summary_text;
ALTER TABLE challenges DROP COLUMN summary_message_id;
ALTER TABLE challenges DROP COLUMN stopped_at;
