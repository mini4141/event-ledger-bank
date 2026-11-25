-- Local equivalent of analytics/athena/daily_volume.sql (reads the sink folder).
SELECT dt, count(*) AS transfers, sum(CAST(amount_minor AS BIGINT)) / 100.0 AS volume_gbp
FROM read_json_auto('.local/sink/**/*.ndjson', hive_partitioning = true)
WHERE topic = 'transfer.completed'
GROUP BY dt ORDER BY dt;
