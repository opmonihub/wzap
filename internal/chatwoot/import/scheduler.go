package chatimport

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/storage"
)

const (
	// DefaultImportInterval is the cron cadence of the lost-messages sync,
	// matching the Evolution syncLostMessages schedule.
	DefaultImportInterval = 30 * time.Minute
	// DefaultLostWindow is the look-back of the lost-messages sync: history
	// newer than this is re-imported, covering messages the live mirror
	// missed.
	DefaultLostWindow = 6 * time.Hour
)

// InstanceLister reads all service instances for a sync cycle.
type InstanceLister interface {
	List(ctx context.Context) ([]model.Instance, error)
}

// ConfigStore reads one instance connector config.
type ConfigStore interface {
	Get(ctx context.Context, id uuid.UUID) (*model.ChatwootConfig, error)
}

// SessionGetter returns the live session of an instance, when one exists.
type SessionGetter interface {
	Get(id uuid.UUID) (session.Session, bool)
}

// SchedulerDeps wires the lost-messages cron. Run imports one instance with
// the window cut (since = now - Window); ClearCache drops the connector
// cache of an imported instance.
type SchedulerDeps struct {
	Interval   time.Duration
	Window     time.Duration
	Instances  InstanceLister
	Configs    ConfigStore
	Sessions   SessionGetter
	Run        func(ctx context.Context, instanceID uuid.UUID, since time.Time) (int, error)
	ClearCache func(instanceID uuid.UUID)
	Log        zerolog.Logger
}

// Scheduler re-imports recent history on an interval, covering the messages
// the live mirror missed. It is safe for concurrent use.
type Scheduler struct {
	interval   time.Duration
	window     time.Duration
	instances  InstanceLister
	configs    ConfigStore
	sessions   SessionGetter
	run        func(ctx context.Context, instanceID uuid.UUID, since time.Time) (int, error)
	clearCache func(instanceID uuid.UUID)
	log        zerolog.Logger
}

// NewScheduler builds a Scheduler over deps, applying the default interval
// and window when unset.
func NewScheduler(deps SchedulerDeps) *Scheduler {
	interval := deps.Interval
	if interval <= 0 {
		interval = DefaultImportInterval
	}
	window := deps.Window
	if window <= 0 {
		window = DefaultLostWindow
	}
	log := deps.Log
	return &Scheduler{
		interval:   interval,
		window:     window,
		instances:  deps.Instances,
		configs:    deps.Configs,
		sessions:   deps.Sessions,
		run:        deps.Run,
		clearCache: deps.ClearCache,
		log:        log,
	}
}

// Start runs sync cycles on the interval until ctx ends. A failed cycle
// warns and the next tick retries; Start never returns an error. Ticks never
// overlap: when a cycle outlasts the interval the tick is skipped with a warn
// instead of running two cycles concurrently over the same feeds.
func (s *Scheduler) Start(ctx context.Context) {
	s.log.Info().Dur("interval", s.interval).Msg("chatwoot import scheduler started")
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	var running atomic.Bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if running.Swap(true) {
				s.log.Warn().Msg("chatwoot sync lost messages skipped: previous cycle still running")
				continue
			}
			if _, err := s.RunOnce(ctx); err != nil {
				s.log.Warn().Err(err).Msg("chatwoot sync lost messages failed")
			}
			running.Store(false)
		}
	}
}

// RunOnce runs one sync cycle over every instance and returns how many
// messages were imported. Instances without connector config, with the
// connector or the message import off, or without a live session are
// skipped; a failing instance warns and the cycle continues with the next
// one. Only an instance listing failure is returned.
func (s *Scheduler) RunOnce(ctx context.Context) (int, error) {
	if s.run == nil || s.instances == nil {
		return 0, nil
	}
	since := time.Now().UTC().Add(-s.window)
	total := 0
	instances, err := s.instances.List(ctx)
	if err != nil {
		return total, fmt.Errorf("chatimport: sync lost messages: list instances: %w", err)
	}
	for _, inst := range instances {
		n, ok := s.syncInstance(ctx, inst.ID, since)
		if !ok {
			continue
		}
		total += n
	}
	return total, nil
}

// syncInstance imports one instance when it is eligible: configured,
// enabled, message import on, and a live session present. The connector
// cache is cleared only after real successful work (imported > 0): an inert
// run (0, nil — no pool from a missing URI, flags off, empty guard) leaves
// the cache, and via RunImport's early returns the accumulators, intact.
// A stale cache is harmless anyway: the mirror rebuilds its stack when the
// connector configuration changes. It reports false for skipped
// and failed instances.
func (s *Scheduler) syncInstance(ctx context.Context, id uuid.UUID, since time.Time) (int, bool) {
	if s.configs == nil || s.sessions == nil {
		return 0, false
	}
	cfg, err := s.configs.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return 0, false
		}
		s.log.Warn().Str("instance_id", id.String()).Err(err).Msg("chatwoot import config lookup failed")
		return 0, false
	}
	if cfg == nil || !cfg.Enabled || !cfg.ImportMessages {
		return 0, false
	}
	if _, ok := s.sessions.Get(id); !ok {
		return 0, false
	}
	n, err := s.run(ctx, id, since)
	if err != nil {
		s.log.Warn().Str("instance_id", id.String()).Err(err).Msg("chatwoot sync lost messages failed")
		return 0, false
	}
	if n > 0 && s.clearCache != nil {
		s.clearCache(id)
	}
	return n, true
}
