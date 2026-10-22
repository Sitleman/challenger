-- +goose Up
CREATE TABLE challenges (
    id               INTEGER PRIMARY KEY,
    chat_id          INTEGER NOT NULL,
    thread_id        INTEGER NOT NULL,
    month            TEXT    NOT NULL,
    stats_message_id INTEGER NOT NULL,
    UNIQUE (chat_id, thread_id, month)
);

CREATE TABLE proofs (
    challenge_id INTEGER NOT NULL REFERENCES challenges (id),
    message_id   INTEGER NOT NULL,
    user_id      INTEGER NOT NULL,
    user_name    TEXT    NOT NULL,
    day          INTEGER NOT NULL,
    PRIMARY KEY (challenge_id, message_id)
);

CREATE TABLE reactions (
    challenge_id INTEGER NOT NULL,
    message_id   INTEGER NOT NULL,
    user_id      INTEGER NOT NULL,
    PRIMARY KEY (challenge_id, message_id, user_id),
    FOREIGN KEY (challenge_id, message_id) REFERENCES proofs (challenge_id, message_id)
);

-- +goose Down
DROP TABLE reactions;
DROP TABLE proofs;
DROP TABLE challenges;
