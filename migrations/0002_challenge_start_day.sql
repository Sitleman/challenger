-- +goose Up
ALTER TABLE challenges ADD COLUMN start_day INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE challenges DROP COLUMN start_day;
