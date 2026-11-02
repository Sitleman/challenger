-- +goose Up
ALTER TABLE challenges ADD COLUMN finish_day INTEGER;
ALTER TABLE challenges ADD COLUMN tg_api_message_id INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE challenges DROP COLUMN tg_api_message_id;
ALTER TABLE challenges DROP COLUMN finish_day;
