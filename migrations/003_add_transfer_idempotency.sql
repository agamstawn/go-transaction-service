BEGIN;

-- Add an optional idempotency key for transfer requests.
-- Existing transfer records remain valid.

ALTER TABLE transfers
ADD COLUMN idempotency_key VARCHAR(128);

-- Reject empty or whitespace-only idempotency keys.

ALTER TABLE transfers
ADD CONSTRAINT transfers_idempotency_key_not_blank
CHECK (
    idempotency_key IS NULL
    OR LENGTH(BTRIM(idempotency_key)) > 0
);

-- Prevent duplicate idempotency keys for the same sender.

CREATE UNIQUE INDEX idx_transfers_sender_idempotency
ON transfers (
    from_account_id,
    idempotency_key
)
WHERE idempotency_key IS NOT NULL;

COMMIT;
