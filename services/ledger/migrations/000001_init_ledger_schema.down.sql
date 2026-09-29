DROP TRIGGER IF EXISTS trg_verify_entries_balance_and_currency ON entries;
DROP FUNCTION IF EXISTS fn_verify_entries_balance_and_currency();

DROP TRIGGER IF EXISTS trg_verify_transaction_min_entries ON transactions;
DROP FUNCTION IF EXISTS fn_verify_transaction_min_entries();

DROP TRIGGER IF EXISTS trg_block_entries_mutation ON entries;
DROP TRIGGER IF EXISTS trg_block_transactions_mutation ON transactions;
DROP FUNCTION IF EXISTS fn_block_mutation();

DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
