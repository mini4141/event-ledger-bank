# Same tables as db/cql/schema.cql. Keyspaces creates tables asynchronously, so Terraform owns DDL
# and the services never run it in AWS.
resource "aws_keyspaces_keyspace" "bank" {
  count = var.enable_keyspaces ? 1 : 0
  name  = "bank"
}

locals {
  keyspaces_tables = {
    customers = {
      columns    = { customer_id = "uuid", name = "text", created_at = "timestamp" }
      partition  = ["customer_id"]
      clustering = {}
    }
    accounts = {
      columns = {
        account_id    = "uuid", customer_id = "uuid", currency = "text",
        balance_minor = "bigint", version = "int", created_at = "timestamp"
      }
      partition  = ["account_id"]
      clustering = {}
    }
    accounts_by_customer = {
      columns    = { customer_id = "uuid", account_id = "uuid" }
      partition  = ["customer_id"]
      clustering = { account_id = "ASC" }
    }
    ledger_entries_by_account = {
      columns = {
        account_id   = "uuid", posted_at = "timeuuid", posting_id = "uuid", direction = "text",
        amount_minor = "bigint", counterparty = "text"
      }
      partition  = ["account_id"]
      clustering = { posted_at = "DESC" }
    }
    transfers_by_idempotency_key = {
      columns    = { idempotency_key = "text", transfer_id = "uuid", status = "text", created_at = "timestamp" }
      partition  = ["idempotency_key"]
      clustering = {}
    }
    postings_by_id = {
      columns    = { posting_id = "uuid", status = "text", created_at = "timestamp" }
      partition  = ["posting_id"]
      clustering = {}
    }
  }
}

resource "aws_keyspaces_table" "t" {
  for_each      = var.enable_keyspaces ? local.keyspaces_tables : {}
  keyspace_name = aws_keyspaces_keyspace.bank[0].name
  table_name    = each.key

  schema_definition {
    dynamic "column" {
      for_each = each.value.columns
      content {
        name = column.key
        type = column.value
      }
    }
    dynamic "partition_key" {
      for_each = each.value.partition
      content {
        name = partition_key.value
      }
    }
    dynamic "clustering_key" {
      for_each = each.value.clustering
      content {
        name     = clustering_key.key
        order_by = clustering_key.value
      }
    }
  }

  capacity_specification {
    throughput_mode = "PAY_PER_REQUEST"
  }
  point_in_time_recovery {
    status = "DISABLED"
  }
}
