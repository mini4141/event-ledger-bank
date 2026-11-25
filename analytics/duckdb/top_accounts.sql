SELECT from_account_id, count(*) AS transfers, sum(CAST(amount_minor AS BIGINT)) / 100.0 AS sent_gbp
FROM read_json_auto('.local/sink/**/*.ndjson', hive_partitioning = true)
WHERE topic = 'transfer.completed'
GROUP BY from_account_id ORDER BY sent_gbp DESC LIMIT 10;
