-- Rollback reports table
DROP INDEX IF EXISTS idx_reports_target;
DROP INDEX IF EXISTS idx_reports_status;
DROP TABLE IF EXISTS reports;
