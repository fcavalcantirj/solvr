DROP TABLE IF EXISTS webhook_deliveries;
ALTER TABLE webhooks DROP COLUMN IF EXISTS signing_secret;
COMMENT ON COLUMN webhooks.events IS 'Array of event types: answer.created, comment.created, approach.stuck, problem.solved, mention';
