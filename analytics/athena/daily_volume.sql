SELECT dt, count(*) AS transfers, sum(amount_minor) / 100.0 AS volume_gbp
FROM events
WHERE topic = 'transfer.completed'
GROUP BY dt ORDER BY dt;
