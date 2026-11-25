-- Local Cassandra schema. In AWS, Terraform creates the same tables in Keyspaces (no DDL from the app).
CREATE KEYSPACE IF NOT EXISTS bank WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};
USE bank;

CREATE TABLE IF NOT EXISTS customers (
  customer_id uuid PRIMARY KEY,
  name text,
  created_at timestamp
);

CREATE TABLE IF NOT EXISTS accounts (
  account_id uuid PRIMARY KEY,
  customer_id uuid,
  currency text,
  balance_minor bigint,
  version int,
  created_at timestamp
);

CREATE TABLE IF NOT EXISTS accounts_by_customer (
  customer_id uuid,
  account_id uuid,
  PRIMARY KEY (customer_id, account_id)
);

CREATE TABLE IF NOT EXISTS ledger_entries_by_account (
  account_id uuid,
  posted_at timeuuid,
  posting_id uuid,
  direction text,
  amount_minor bigint,
  counterparty text,
  PRIMARY KEY (account_id, posted_at)
) WITH CLUSTERING ORDER BY (posted_at DESC);

CREATE TABLE IF NOT EXISTS transfers_by_idempotency_key (
  idempotency_key text PRIMARY KEY,
  transfer_id uuid,
  status text,
  created_at timestamp
);

CREATE TABLE IF NOT EXISTS postings_by_id (
  posting_id uuid PRIMARY KEY,
  status text,
  created_at timestamp
);
