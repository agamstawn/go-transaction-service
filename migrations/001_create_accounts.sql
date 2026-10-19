BEGIN;

CREATE TABLE accounts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    account_number VARCHAR(32) NOT NULL UNIQUE,

    currency VARCHAR(3) NOT NULL DEFAULT 'IDR',

    balance BIGINT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT accounts_number_not_blank
        CHECK (LENGTH(BTRIM(account_number)) > 0),

    CONSTRAINT accounts_currency_idr
        CHECK (currency = 'IDR'),

    CONSTRAINT accounts_balance_nonnegative
        CHECK (balance >= 0)
);

COMMIT;
