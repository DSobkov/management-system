BEGIN;

CREATE TABLE IF NOT EXISTS user_acsess_tokens (
    id SERIAL PRIMARY KEY REFERENCES users(id),
    acsess_token VARCHAR(255) NOT NULL,
	generated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL
);


COMMIT;