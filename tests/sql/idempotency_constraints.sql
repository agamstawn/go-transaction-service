BEGIN;

DO $$
DECLARE
    sender_id BIGINT;
    recipient_id BIGINT;
    another_sender_id BIGINT;
    test_key TEXT := 'idem-test-' || txid_current()::TEXT;
BEGIN

    -- Create test accounts.
    INSERT INTO accounts (account_number, balance)
    VALUES ('idem-src-' || txid_current()::TEXT, 500000)
    RETURNING id INTO sender_id;

    INSERT INTO accounts (account_number, balance)
    VALUES ('idem-dst-' || txid_current()::TEXT, 200000)
    RETURNING id INTO recipient_id;

    INSERT INTO accounts (account_number, balance)
    VALUES ('idem-alt-' || txid_current()::TEXT, 300000)
    RETURNING id INTO another_sender_id;

    -- 1. First request with a new idempotency key.
    INSERT INTO transfers (
        from_account_id,
        to_account_id,
        amount,
        idempotency_key
    )
    VALUES (
        sender_id,
        recipient_id,
        125000,
        test_key
    );

    RAISE NOTICE 'PASS: first keyed transfer recorded';

    -- 2. Duplicate key from the same sender.
    BEGIN
        INSERT INTO transfers (
            from_account_id,
            to_account_id,
            amount,
            idempotency_key
        )
        VALUES (
            sender_id,
            recipient_id,
            125000,
            test_key
        );

        RAISE EXCEPTION 'FAIL: duplicate key accepted';

    EXCEPTION
        WHEN unique_violation THEN
            RAISE NOTICE 'PASS: duplicate key rejected';
    END;

    -- 3. Same key from a different sender is allowed.
    INSERT INTO transfers (
        from_account_id,
        to_account_id,
        amount,
        idempotency_key
    )
    VALUES (
        another_sender_id,
        recipient_id,
        10000,
        test_key
    );

    RAISE NOTICE 'PASS: key scoped by sender';

    -- 4. Blank idempotency key must be rejected.
    BEGIN
        INSERT INTO transfers (
            from_account_id,
            to_account_id,
            amount,
            idempotency_key
        )
        VALUES (
            sender_id,
            recipient_id,
            10000,
            '   '
        );

        RAISE EXCEPTION 'FAIL: blank key accepted';

    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'PASS: blank key rejected';
    END;

    -- 5. Legacy transfers without a key remain supported.
    INSERT INTO transfers (
        from_account_id,
        to_account_id,
        amount
    )
    VALUES (
        sender_id,
        recipient_id,
        10000
    );

    RAISE NOTICE 'PASS: legacy transfer supported';

END
$$;

ROLLBACK;
