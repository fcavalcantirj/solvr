-- The served duration leaves api_request_events; every other column is untouched.
ALTER TABLE api_request_events DROP CONSTRAINT IF EXISTS api_request_events_duration_ms_check;
ALTER TABLE api_request_events DROP COLUMN IF EXISTS duration_ms;
