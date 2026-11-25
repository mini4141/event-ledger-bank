# Cassandra data model

Cassandra tables are designed query-first: one table per read path, partitioned so each query hits a single partition.

| Query | Table | Partition key | Clustering |
|---|---|---|---|
| Get account / balance | `accounts` | `account_id` | none |
| List a customer's accounts | `accounts_by_customer` | `customer_id` | `account_id` |
| Account statement, newest first | `ledger_entries_by_account` | `account_id` | `posted_at` (timeuuid) DESC |
| Idempotency check for a transfer | `transfers_by_idempotency_key` | `idempotency_key` | none |
| Dedupe a ledger posting (Envoy retries) | `postings_by_id` | `posting_id` | none |
| List customers (admin tool only) | `customers` | `customer_id` | none, full scan with LIMIT |

## Concurrency and idempotency

- **Balances** are updated with a lightweight transaction: `UPDATE accounts SET balance_minor=?, version=? WHERE account_id=? IF version=?`. On a version conflict the ledger re-reads and retries (up to 8 times, with a short backoff).
- **Transfers** claim the idempotency key with `INSERT ... IF NOT EXISTS`. A replay returns the stored `transfer_id` and never calls the ledger again.
- **Postings** are marked `POSTING` with `INSERT ... IF NOT EXISTS`, so a retried `ledger.Post` cannot post twice. The marker moves to `POSTED` (or `REJECTED`) at the end.
- A posting is two balance updates plus two entry rows sharing one `posting_id`: the debit comes first and the credit is retried. If the credit cannot be applied, the debit is reversed.

## Known limits (out of scope)

- No cross-row atomic transaction: a crash between the debit and the credit leaves the posting in `POSTING` and needs a recovery job. A real bank would use a saga or outbox.
- Event publishing to Kafka is best effort after the commit (no transactional outbox).
- `ListCustomers` does a full table scan, which is fine for an internal admin tool at demo scale.

## Amazon Keyspaces

The same tables are created by Terraform (`infra/keyspaces.tf`): Keyspaces creates tables asynchronously, so the app never runs DDL there. Connections use TLS on port 9142, SigV4 auth through Pod Identity credentials, and `LOCAL_QUORUM`.
