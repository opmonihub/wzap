-- +goose Up
-- remodel_report gains the explicit-resolution marker the cutover gate
-- (00010) requires: a device_jid_conflicts row with resolved_at set records
-- that an operator reconciled the identity and acknowledged the divergence.
-- The column lives in its own migration so environments that already applied
-- the original 00008 receive it too — goose never re-runs an applied
-- migration, so 00008 must stay untouched — and so the marker survives a
-- failed gate attempt: 00010 aborts and rolls back on unresolved conflicts,
-- but the operator loop that follows needs this column to record the
-- resolution before rerunning wzap migrate.

ALTER TABLE remodel_report ADD COLUMN resolved_at timestamptz;

-- +goose Down
ALTER TABLE remodel_report DROP COLUMN resolved_at;