package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"wzap/internal/model"
	"wzap/internal/storage"
	"wzap/internal/webhook"
)

// instanceColumns reads the identity row joined with its connection, webhook
// and chat-settings satellites. The satellites are created with the identity
// or on first write of their concern, so the LEFT JOINs pair or read NULL; the
// COALESCEs only defend against rows inserted by direct SQL outside the
// repository (tests seeding minimal identities).
const instanceColumns = `i.id, i.name, COALESCE(i.external_ref, '') AS external_ref, ` +
	`COALESCE(c.device_jid, '') AS device_jid, COALESCE(c.status, 'disconnected') AS status, ` +
	`c.last_connected_at, c.last_error_code, c.last_error_message, c.last_error_at, ` +
	`i.owner_user_id, w.url AS webhook_url, ` +
	`COALESCE(w.is_enabled, false) AS webhook_enabled, ` +
	`COALESCE(w.events, '{message,receipt,connection,message.status}'::text[]) AS webhook_events, ` +
	`s.default_disappearing_seconds, ` +
	`i.created_at, i.updated_at`

const instanceJoin = ` FROM instances i ` +
	`LEFT JOIN instance_connections c ON c.instance_id = i.id ` +
	`LEFT JOIN instance_webhooks w ON w.instance_id = i.id ` +
	`LEFT JOIN instance_chat_settings s ON s.instance_id = i.id`

const listInstancesQuery = `SELECT ` + instanceColumns + instanceJoin +
	` ORDER BY i.created_at DESC, i.id DESC`

// InstanceRepository is the pgx-backed storage.InstanceRepository.
type InstanceRepository struct {
	pool *pgxpool.Pool
}

var _ storage.InstanceRepository = (*InstanceRepository)(nil)

// NewInstanceRepository returns an instance repository backed by pool.
func NewInstanceRepository(pool *pgxpool.Pool) *InstanceRepository {
	return &InstanceRepository{pool: pool}
}

// Create persists the identity row plus its connection and webhook
// satellites in one transaction, and returns the aggregate with database
// timestamps. A nil OwnerUserID stores NULL (legacy rows); the service always
// supplies an owner for new rows. A nil Webhook.URL stores NULL (unset) and a
// nil Events slice falls back to the canonical default, matching the
// migration defaults.
func (r *InstanceRepository) Create(ctx context.Context, instance model.Instance) (*model.Instance, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, mapInstanceError("create instance", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := claimInstanceName(ctx, tx, instance.Name, nil); err != nil {
		return nil, mapInstanceError("create instance", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO instances (id, name, external_ref, owner_user_id)
		VALUES ($1, $2, NULLIF($3, ''), $4)`,
		instance.ID, instance.Name, instance.ExternalRef, instance.OwnerUserID,
	); err != nil {
		return nil, mapInstanceError("create instance", err)
	}

	status := instance.Connection.Status
	if status == "" {
		status = "disconnected"
	}
	var lastErrCode, lastErrMessage any
	var lastErrAt *time.Time
	if instance.Connection.LastError != nil {
		lastErrCode = nullString(instance.Connection.LastError.Code)
		lastErrMessage = nullString(instance.Connection.LastError.Message)
		lastErrAt = instance.Connection.LastError.At
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO instance_connections
			(instance_id, device_jid, status, last_connected_at,
			 last_error_code, last_error_message, last_error_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6, $7)`,
		instance.ID, instance.Connection.DeviceJID, status,
		instance.Connection.LastConnectedAt, lastErrCode, lastErrMessage, lastErrAt,
	); err != nil {
		return nil, mapInstanceError("create instance", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO instance_webhooks (instance_id, url, is_enabled, events)
		VALUES ($1, NULLIF($2, ''), $3, $4)`,
		instance.ID, webhookURLParam(instance.Webhook.URL),
		instance.Webhook.IsEnabled, webhookEventsParam(instance.Webhook.Events),
	); err != nil {
		return nil, mapInstanceError("create instance", err)
	}

	created, err := scanInstance(tx.QueryRow(ctx,
		`SELECT `+instanceColumns+instanceJoin+` WHERE i.id = $1`, instance.ID))
	if err != nil {
		return nil, mapInstanceError("create instance", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapInstanceError("create instance", err)
	}
	return created, nil
}

// Get returns the instance aggregate with the given id or storage.ErrNotFound.
func (r *InstanceRepository) Get(ctx context.Context, id uuid.UUID) (*model.Instance, error) {
	instance, err := scanInstance(r.pool.QueryRow(ctx,
		`SELECT `+instanceColumns+instanceJoin+` WHERE i.id = $1`, id))
	if err != nil {
		return nil, mapInstanceError("get instance", err)
	}
	return instance, nil
}

// GetByName returns the unique exact match, rejecting ambiguous legacy rows.
func (r *InstanceRepository) GetByName(ctx context.Context, name string) (*model.Instance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+instanceColumns+instanceJoin+` WHERE i.name = $1 LIMIT 2`, name)
	if err != nil {
		return nil, fmt.Errorf("get instance by name: %w", err)
	}
	defer rows.Close()
	var found *model.Instance
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("get instance by name: %w", err)
		}
		if found != nil {
			return nil, fmt.Errorf("get instance by name: %w", storage.ErrInstanceNameAmbiguous)
		}
		found = instance
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get instance by name: %w", err)
	}
	if found == nil {
		return nil, fmt.Errorf("get instance by name: %w", storage.ErrNotFound)
	}
	return found, nil
}

// GetByDeviceJID returns the instance bound to deviceJID or storage.ErrNotFound.
func (r *InstanceRepository) GetByDeviceJID(ctx context.Context, deviceJID string) (*model.Instance, error) {
	deviceJID = stringsTrim(deviceJID)
	if deviceJID == "" {
		return nil, fmt.Errorf("get instance by device jid: %w", storage.ErrNotFound)
	}
	instance, err := scanInstance(r.pool.QueryRow(ctx,
		`SELECT `+instanceColumns+instanceJoin+` WHERE c.device_jid = $1`, deviceJID))
	if err != nil {
		return nil, mapInstanceError("get instance by device jid", err)
	}
	return instance, nil
}

// GetByExternalRef returns the instance with the given external_ref or
// storage.ErrNotFound.
func (r *InstanceRepository) GetByExternalRef(ctx context.Context, externalRef string) (*model.Instance, error) {
	instance, err := scanInstance(r.pool.QueryRow(ctx,
		`SELECT `+instanceColumns+instanceJoin+` WHERE i.external_ref = $1`, externalRef))
	if err != nil {
		return nil, mapInstanceError("get instance by external ref", err)
	}
	return instance, nil
}

// List returns every instance aggregate ordered by created_at descending,
// then id descending.
func (r *InstanceRepository) List(ctx context.Context) ([]model.Instance, error) {
	rows, err := r.pool.Query(ctx, listInstancesQuery)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	instances := []model.Instance{}
	for rows.Next() {
		var instance model.Instance
		if err := scanInstanceRow(rows, &instance); err != nil {
			return nil, fmt.Errorf("list instances: %w", err)
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	return instances, nil
}

// UpdateIdentity rewrites only the identity columns (name, external_ref) of
// the instance and returns the re-read aggregate. The name-claim protocol is
// unchanged: the current name is read under a row lock so a stale caller
// cannot bypass the claim after a concurrent rename.
func (r *InstanceRepository) UpdateIdentity(ctx context.Context, id uuid.UUID, name, externalRef string) (*model.Instance, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, mapInstanceError("update instance", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Read the current database name under a row lock: a stale caller must not
	// bypass the claim after another update has renamed this same instance.
	var currentName string
	if err := tx.QueryRow(ctx, `SELECT name FROM instances WHERE id = $1 FOR UPDATE`, id).Scan(&currentName); err != nil {
		return nil, mapInstanceError("update instance", err)
	}
	if name != currentName {
		if !model.IsValidInstanceName(name) {
			return nil, mapInstanceError("update instance", storage.ErrInvalidInstanceName)
		}
		if err := claimInstanceName(ctx, tx, name, &id); err != nil {
			return nil, mapInstanceError("update instance", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE instances
		SET name = $2, external_ref = NULLIF($3, ''), updated_at = now()
		WHERE id = $1`,
		id, name, externalRef,
	); err != nil {
		return nil, mapInstanceError("update instance", err)
	}
	updated, err := scanInstance(tx.QueryRow(ctx,
		`SELECT `+instanceColumns+instanceJoin+` WHERE i.id = $1`, id))
	if err != nil {
		return nil, mapInstanceError("update instance", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapInstanceError("update instance", err)
	}
	return updated, nil
}

// claimInstanceName serializes participating writers by schema and exact name.
// The occupancy query must be a separate statement after the lock: under READ
// COMMITTED it sees the winning writer's commit even when lock acquisition waits.
// Direct SQL and older writers do not participate in this protocol.
func claimInstanceName(ctx context.Context, tx pgx.Tx, name string, excludeID *uuid.UUID) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(current_schema() || ':' || $1, 0))`, name); err != nil {
		return err
	}
	var occupied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM instances WHERE name = $1 AND ($2::uuid IS NULL OR id <> $2))`, name, excludeID).Scan(&occupied); err != nil {
		return err
	}
	if occupied {
		return storage.ErrInstanceNameTaken
	}
	return nil
}

// SetConnection updates the connection row of an instance and clears the
// bound device JID when deviceJID is empty. It leaves the identity and
// webhook tables untouched.
func (r *InstanceRepository) SetConnection(ctx context.Context, id uuid.UUID, status, deviceJID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE instance_connections
		SET status = $2,
		    device_jid = NULLIF($3, ''),
		    last_error_code = CASE WHEN NULLIF($3, '') IS NULL THEN NULL ELSE last_error_code END,
		    last_error_message = CASE WHEN NULLIF($3, '') IS NULL THEN NULL ELSE last_error_message END,
		    last_error_at = CASE WHEN NULLIF($3, '') IS NULL THEN NULL ELSE last_error_at END,
		    updated_at = now()
		WHERE instance_id = $1`,
		id, status, deviceJID,
	)
	if err != nil {
		return mapInstanceError("set instance connection", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set instance connection: %w", storage.ErrNotFound)
	}
	return nil
}

// SetConnectionState records a connection transition on the connection row of
// an instance without touching identity columns: status and the error trio
// always, device_jid when it is not empty (keeping the stored one otherwise)
// and last_connected_at when connectedAt is set. An empty lastError clears
// the trio; a non-empty one stores the classified code with the current
// instant.
func (r *InstanceRepository) SetConnectionState(ctx context.Context, id uuid.UUID, status, deviceJID, lastError string, connectedAt *time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE instance_connections
		SET status = $2,
		    device_jid = COALESCE(NULLIF($3, ''), device_jid),
		    last_error_code = CASE WHEN $4 = '' THEN NULL ELSE $5 END,
		    last_error_message = NULLIF($4, ''),
		    last_error_at = CASE WHEN $4 = '' THEN NULL ELSE now() END,
		    last_connected_at = COALESCE($6, last_connected_at),
		    updated_at = now()
		WHERE instance_id = $1`,
		id, status, deviceJID, lastError, connectionErrorCode(lastError), connectedAt,
	)
	if err != nil {
		return mapInstanceError("set instance connection state", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set instance connection state: %w", storage.ErrNotFound)
	}
	return nil
}

// SetWebhook replaces the webhook configuration of an instance, never
// touching the identity or connection tables. A nil URL stores NULL (unset)
// and a nil events slice falls back to the canonical default.
func (r *InstanceRepository) SetWebhook(ctx context.Context, id uuid.UUID, url *string, enabled bool, events []string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE instance_webhooks
		SET url = NULLIF($2, ''), is_enabled = $3, events = $4, updated_at = now()
		WHERE instance_id = $1`,
		id, webhookURLParam(url), enabled, webhookEventsParam(events),
	)
	if err != nil {
		return fmt.Errorf("set instance webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set instance webhook: %w", storage.ErrNotFound)
	}
	return nil
}

// SetDefaultDisappearing upserts the persisted echo of the default
// disappearing timer on the instance_chat_settings satellite, never touching
// identity, connection or webhook columns. The row is created on first write;
// a zero duration stores the off value (not NULL: NULL means never
// configured). It reports storage.ErrNotFound when the instance does not
// exist.
func (r *InstanceRepository) SetDefaultDisappearing(ctx context.Context, id uuid.UUID, duration time.Duration) error {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO instance_chat_settings (instance_id, default_disappearing_seconds)
		VALUES ($1, $2)
		ON CONFLICT (instance_id) DO UPDATE
		SET default_disappearing_seconds = EXCLUDED.default_disappearing_seconds,
		    updated_at = now()`,
		id, int64(duration/time.Second),
	)
	if err != nil {
		return mapInstanceError("set instance default disappearing", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set instance default disappearing: %w", storage.ErrNotFound)
	}
	return nil
}

// BackfillOwner claims every legacy instance with a NULL owner for owner and
// returns how many rows were claimed. Instances that already have an owner are
// never touched: ownership is immutable.
func (r *InstanceRepository) BackfillOwner(ctx context.Context, owner uuid.UUID) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE instances
		SET owner_user_id = $1, updated_at = now()
		WHERE owner_user_id IS NULL`,
		owner,
	)
	if err != nil {
		return 0, fmt.Errorf("backfill instance owners: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Delete removes the instance and its dependent rows, or storage.ErrNotFound.
func (r *InstanceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM instances WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete instance: %w", storage.ErrNotFound)
	}
	return nil
}

// connectionErrorCode classifies a free-text connection failure into the
// catalog code stored alongside it. New writers always record the typed code
// at the source; unrecognized reasons keep upstream_error instead of guessing.
func connectionErrorCode(reason string) string {
	if reason == "" {
		return ""
	}
	switch {
	case strings.Contains(reason, "logged out:"):
		return "logged_out"
	case strings.Contains(reason, "stream replaced"):
		return "stream_replaced"
	case strings.Contains(reason, "device jid mismatch"):
		return "device_jid_mismatch"
	case strings.Contains(reason, "device jid already bound"):
		return "device_jid_taken"
	case strings.Contains(reason, "the session was rejected"):
		return "session_rejected"
	default:
		return "upstream_error"
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanInstance(scanner rowScanner) (*model.Instance, error) {
	var instance model.Instance
	if err := scanInstanceRow(scanner, &instance); err != nil {
		return nil, err
	}
	return &instance, nil
}

func scanInstanceRow(scanner rowScanner, instance *model.Instance) error {
	var (
		deviceJID                  string
		status                     string
		lastConnectedAt            *time.Time
		lastErrCode                *string
		lastErrMessage             *string
		lastErrAt                  *time.Time
		webhookURL                 *string
		webhookEnabled             bool
		webhookEvents              []string
		defaultDisappearingSeconds *int64
	)
	if err := scanner.Scan(
		&instance.ID, &instance.Name, &instance.ExternalRef,
		&deviceJID, &status, &lastConnectedAt, &lastErrCode, &lastErrMessage, &lastErrAt,
		&instance.OwnerUserID, &webhookURL, &webhookEnabled, &webhookEvents,
		&defaultDisappearingSeconds,
		&instance.CreatedAt, &instance.UpdatedAt,
	); err != nil {
		return err
	}
	instance.Connection = model.InstanceConnection{
		InstanceID:      instance.ID,
		DeviceJID:       deviceJID,
		Status:          status,
		LastConnectedAt: lastConnectedAt,
	}
	if lastErrCode != nil || lastErrMessage != nil {
		instance.Connection.LastError = &model.InstanceError{
			Code:    derefString(lastErrCode),
			Message: derefString(lastErrMessage),
			At:      lastErrAt,
		}
	}
	instance.Webhook = model.InstanceWebhook{
		InstanceID: instance.ID,
		URL:        webhookURL,
		IsEnabled:  webhookEnabled,
		Events:     webhookEvents,
	}
	if defaultDisappearingSeconds != nil {
		duration := time.Duration(*defaultDisappearingSeconds) * time.Second
		instance.DefaultDisappearing = &duration
	} else {
		instance.DefaultDisappearing = nil
	}
	return nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// nullString maps an empty string to a SQL NULL parameter.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// webhookURLParam maps an unset webhook URL to NULL for the nullable column.
func webhookURLParam(url *string) any {
	if url == nil {
		return nil
	}
	return *url
}

// webhookEventsParam keeps the NOT NULL events column satisfied: an explicit
// empty subscription stores an empty array, while a nil slice (no writer ever
// produces one — the service defaults absent events) falls back to the
// canonical default instead of violating the constraint.
func webhookEventsParam(events []string) []string {
	if events == nil {
		return webhook.DefaultEvents()
	}
	return events
}

func mapInstanceError(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, storage.ErrNotFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			switch pgErr.ConstraintName {
			case "instances_external_ref_key":
				return fmt.Errorf("%s: %w", op, storage.ErrExternalRefTaken)
			case "instance_connections_device_jid_uidx":
				return fmt.Errorf("%s: %w", op, storage.ErrDeviceJIDTaken)
			}
		case "23503":
			// A foreign-key violation on a satellite write means the owning
			// instance row is gone: the caller sees ErrNotFound.
			return fmt.Errorf("%s: %w", op, storage.ErrNotFound)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func stringsTrim(s string) string {
	return strings.TrimSpace(s)
}
