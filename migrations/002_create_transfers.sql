BEGIN;

CREATE TABLE transfers (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    from_account_id BIGINT NOT NULL,

    to_account_id BIGINT NOT NULL,

    amount BIGINT NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'completed',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT transfers_from_account_fk
        FOREIGN KEY (from_account_id)
        REFERENCES accounts(id),

    CONSTRAINT transfers_to_account_fk
        FOREIGN KEY (to_account_id)
        REFERENCES accounts(id),

    CONSTRAINT transfers_positive_amount
        CHECK (amount > 0),

    CONSTRAINT transfers_different_accounts
        CHECK (from_account_id <> to_account_id),

    CONSTRAINT transfers_valid_status
        CHECK (status = 'completed')
);

CREATE INDEX idx_transfers_from_account_created_at
    ON transfers (from_account_id, created_at DESC);

CREATE INDEX idx_transfers_to_account_created_at
    ON transfers (to_account_id, created_at DESC);

COMMIT;
