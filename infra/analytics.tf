# analytics-sink writes NDJSON to S3; Glue + Athena query it (the AWS equivalent of the data stack).
resource "aws_s3_bucket" "data" {
  bucket        = "${var.name}-data-${local.account_id}"
  force_destroy = true
}

resource "aws_s3_bucket" "athena" {
  bucket        = "${var.name}-athena-${local.account_id}"
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "block" {
  for_each                = { data = aws_s3_bucket.data.id, athena = aws_s3_bucket.athena.id }
  bucket                  = each.value
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_glue_catalog_database" "bank" {
  name = "bank"
}

resource "aws_glue_catalog_table" "events" {
  database_name = aws_glue_catalog_database.bank.name
  name          = "events"
  table_type    = "EXTERNAL_TABLE"

  parameters = {
    EXTERNAL                      = "TRUE"
    classification                = "json"
    "projection.enabled"          = "true"
    "projection.dt.type"          = "date"
    "projection.dt.format"        = "yyyy-MM-dd"
    "projection.dt.range"         = "2026-01-01,NOW"
    "projection.dt.interval"      = "1"
    "projection.dt.interval.unit" = "DAYS"
    "projection.hour.type"        = "integer"
    "projection.hour.range"       = "0,23"
    "projection.hour.digits"      = "2"
    "storage.location.template"   = "s3://${aws_s3_bucket.data.bucket}/events/dt=$${dt}/hour=$${hour}"
  }

  partition_keys {
    name = "dt"
    type = "string"
  }
  partition_keys {
    name = "hour"
    type = "string"
  }

  storage_descriptor {
    location      = "s3://${aws_s3_bucket.data.bucket}/events/"
    input_format  = "org.apache.hadoop.mapred.TextInputFormat"
    output_format = "org.apache.hadoop.hive.ql.io.HiveIgnoreKeyTextOutputFormat"

    ser_de_info {
      serialization_library = "org.openx.data.jsonserde.JsonSerDe"
      parameters            = { "ignore.malformed.json" = "true" }
    }

    dynamic "columns" {
      for_each = {
        topic           = "string"
        key             = "string"
        posting_id      = "string"
        transfer_id     = "string"
        account_id      = "string"
        from_account_id = "string"
        to_account_id   = "string"
        counterparty    = "string"
        direction       = "string"
        amount_minor    = "bigint"
        reason          = "string"
        occurred_at     = "string"
      }
      content {
        name = columns.key
        type = columns.value
      }
    }
  }
}

resource "aws_athena_workgroup" "bank" {
  name          = var.name
  force_destroy = true

  configuration {
    enforce_workgroup_configuration = true
    bytes_scanned_cutoff_per_query  = 1073741824 # 1 GB
    result_configuration {
      output_location = "s3://${aws_s3_bucket.athena.bucket}/results/"
    }
  }
}
