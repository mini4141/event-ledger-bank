SELECT from_account_id, count(*) AS transfers, sum(amount_minor) / 100.0 AS sent_gbp
FROM events
WHERE topic = 'transfer.completed'
GROUP BY from_account_id ORDER BY sent_gbp DESC LIMIT 10;
