BEGIN;

-- Valid accounts
INSERT INTO accounts (
    account_number,
    currency,
    balance
)
VALUES
    ('10000001', 'IDR', 500000),
    ('10000002', 'IDR', 0);

-- Invalid cases must be rejected.
DO $$
BEGIN
    -- Negative balance
    BEGIN
        INSERT INTO accounts (
            account_number, balance
        )
        VALUES ('10000003', -10000);

        RAISE EXCEPTION 'Negative balance was accepted';
    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'PASS: negative balance rejected';
    END;

    -- Duplicate account number
    BEGIN
        INSERT INTO accounts (
            account_number, balance
        )
        VALUES ('10000001', 100000);

        RAISE EXCEPTION 'Duplicate account was accepted';
    EXCEPTION
        WHEN unique_violation THEN
            RAISE NOTICE 'PASS: duplicate account rejected';
    END;

    -- Unsupported currency
    BEGIN
        INSERT INTO accounts (
            account_number, currency, balance
        )
        VALUES ('10000004', 'USD', 100000);

        RAISE EXCEPTION 'Unsupported currency was accepted';
    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'PASS: unsupported currency rejected';
    END;

    -- Blank account number
    BEGIN
        INSERT INTO accounts (
            account_number, balance
        )
        VALUES ('   ', 100000);

        RAISE EXCEPTION 'Blank account number was accepted';
    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'PASS: blank account number rejected';
    END;
END
$$;

ROLLBACK;
