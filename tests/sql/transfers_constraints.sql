BEGIN;

DO $$
DECLARE
    sender_id BIGINT;
    recipient_id BIGINT;
    transfer_id BIGINT;
    recorded_status TEXT;
BEGIN

    -- Create temporary test accounts.
    INSERT INTO accounts (
        account_number,
        currency,
        balance
    )
    VALUES (
        'sql-src-' || txid_current()::TEXT,
        'IDR',
        500000
    )
    RETURNING id INTO sender_id;

    INSERT INTO accounts (
        account_number,
        currency,
        balance
    )
    VALUES (
        'sql-dst-' || txid_current()::TEXT,
        'IDR',
        200000
    )
    RETURNING id INTO recipient_id;

    -- 1. Valid transfer
    INSERT INTO transfers (
        from_account_id,
        to_account_id,
        amount
    )
    VALUES (
        sender_id,
        recipient_id,
        125000
    )
    RETURNING id INTO transfer_id;

    SELECT status
    INTO recorded_status
    FROM transfers
    WHERE id = transfer_id;

    IF recorded_status IS DISTINCT FROM 'completed' THEN
        RAISE EXCEPTION 'Unexpected transfer status';
    END IF;

    RAISE NOTICE 'PASS: valid transfer recorded';

    -- 2. Zero amount
    BEGIN
        INSERT INTO transfers (
            from_account_id,
            to_account_id,
            amount
        )
        VALUES (sender_id, recipient_id, 0);

        RAISE EXCEPTION 'FAIL: zero amount was accepted';

    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'PASS: zero amount rejected';
    END;

    -- 3. Negative amount
    BEGIN
        INSERT INTO transfers (
            from_account_id,
            to_account_id,
            amount
        )
        VALUES (sender_id, recipient_id, -10000);

        RAISE EXCEPTION 'FAIL: negative amount was accepted';

    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'PASS: negative amount rejected';
    END;

    -- 4. Same source and destination
    BEGIN
        INSERT INTO transfers (
            from_account_id,
            to_account_id,
            amount
        )
        VALUES (sender_id, sender_id, 10000);

        RAISE EXCEPTION 'FAIL: same account transfer accepted';

    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'PASS: same account transfer rejected';
    END;

    -- 5. Unknown sender
    BEGIN
        INSERT INTO transfers (
            from_account_id,
            to_account_id,
            amount
        )
        VALUES (-1, recipient_id, 10000);

        RAISE EXCEPTION 'FAIL: unknown sender accepted';

    EXCEPTION
        WHEN foreign_key_violation THEN
            RAISE NOTICE 'PASS: unknown sender rejected';
    END;

    -- 6. Unsupported status
    BEGIN
        INSERT INTO transfers (
            from_account_id,
            to_account_id,
            amount,
            status
        )
        VALUES (
            sender_id,
            recipient_id,
            10000,
            'unknown'
        );

        RAISE EXCEPTION 'FAIL: invalid status accepted';

    EXCEPTION
        WHEN check_violation THEN
            RAISE NOTICE 'PASS: invalid status rejected';
    END;

END
$$;

ROLLBACK;
