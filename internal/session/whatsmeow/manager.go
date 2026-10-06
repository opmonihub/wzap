// Package whatsmeow implements the session contracts on top of the upstream
// go.mau.fi/whatsmeow library. All library types stay confined to this package.
package whatsmeow

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/storage"
)

const (
	// restoreConcurrency limits how many persisted sessions reconnect at once.
	restoreConcurrency = 2
	// restoreJitter spreads reconnections so they do not hit the server
	// together.
	restoreJitter = 500 * time.Millisecond
)

// Manager owns the whatsmeow client of every instance and persists the
// sessions in the Postgres device store.
type Manager struct {
	devices       *sqlstore.Container
	instances     storage.InstanceRepository
	log           zerolog.Logger
	sink          session.EventSink
	maxMediaBytes int64

	// restoreConnect brings a restored session online; tests replace it to
	// avoid the network handshake.
	restoreConnect func(ctx context.Context, sess *instanceSession) error

	mu       sync.RWMutex
	sessions map[uuid.UUID]*instanceSession
}

var _ session.Manager = (*Manager)(nil)

// NewManager opens the whatsmeow device store in the Postgres database
// addressed by databaseURL. instances lets RestoreAll map persisted devices
// back to their instance, sink receives the session events and maxMediaBytes
// caps how much inbound media a download may buffer.
func NewManager(ctx context.Context, databaseURL string, instances storage.InstanceRepository, log zerolog.Logger, sink session.EventSink, maxMediaBytes int64) (*Manager, error) {
	devices, err := openDeviceStore(ctx, databaseURL, log)
	if err != nil {
		return nil, err
	}
	manager := &Manager{
		devices:       devices,
		instances:     instances,
		log:           log,
		sink:          sink,
		maxMediaBytes: maxMediaBytes,
		sessions:      make(map[uuid.UUID]*instanceSession),
	}
	manager.restoreConnect = func(ctx context.Context, sess *instanceSession) error {
		return sess.connectExisting(ctx)
	}
	return manager, nil
}

// Close releases the whatsmeow database connections.
func (m *Manager) Close() error {
	if m == nil || m.devices == nil {
		return nil
	}
	return m.devices.Close()
}

// Get returns the session of instanceID when it is known.
func (m *Manager) Get(instanceID uuid.UUID) (session.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[instanceID]
	if !ok {
		return nil, false
	}
	return sess, true
}

// Create returns the session of instance, loading the persisted device when
// the instance already has a whatsapp_jid and building a fresh device for a
// new pairing otherwise. Repeating it for a known instance returns the
// existing session.
func (m *Manager) Create(instance *model.Instance) (session.Session, error) {
	if instance == nil {
		return nil, errors.New("create session: nil instance")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.sessions[instance.ID]; ok {
		return sess, nil
	}

	device, err := m.deviceFor(context.Background(), instance)
	if err != nil {
		return nil, err
	}
	sess, err := newSession(instance.ID, device, m.log, m.sink, m.maxMediaBytes)
	if err != nil {
		return nil, err
	}
	m.sessions[instance.ID] = sess
	return sess, nil
}

// Remove disconnects the session and deletes its stored credentials. When the
// deletion fails the session stays registered, so a retry can finish removing
// the credentials instead of silently reporting success.
func (m *Manager) Remove(ctx context.Context, instanceID uuid.UUID) error {
	m.mu.Lock()
	sess, ok := m.sessions[instanceID]
	if ok {
		delete(m.sessions, instanceID)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}

	if err := sess.remove(ctx); err != nil {
		m.mu.Lock()
		if _, exists := m.sessions[instanceID]; !exists {
			m.sessions[instanceID] = sess
		}
		m.mu.Unlock()
		return err
	}
	return nil
}

// RestoreAll reconnects every persisted session, at most restoreConcurrency at
// a time and with jitter. Failures are reported per instance through the event
// sink; only listing the instances can fail the call.
func (m *Manager) RestoreAll(ctx context.Context) error {
	if m.instances == nil {
		return errors.New("restore sessions: instance repository not configured")
	}
	instances, err := m.listInstances(ctx)
	if err != nil {
		return err
	}

	m.log.Info().Int("total", len(instances)).Msg("restore sessions started")
	sem := make(chan struct{}, restoreConcurrency)
	var wg sync.WaitGroup
	for _, instance := range instances {
		if instance.BoundDeviceJID() == "" {
			m.log.Debug().Str("instance_id", instance.ID.String()).Str("reason", "no-jid").Msg("restore skipped")
			continue
		}
		if _, ok := m.Get(instance.ID); ok {
			m.log.Debug().Str("instance_id", instance.ID.String()).Str("reason", "already-registered").Msg("restore skipped")
			continue
		}
		wg.Add(1)
		go func(instance model.Instance) {
			defer wg.Done()
			if err := ctx.Err(); err != nil {
				m.restoreAborted(instance, err)
				return
			}
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				m.restoreAborted(instance, ctx.Err())
				return
			}
			defer func() { <-sem }()

			if err := sleepCtx(ctx, time.Duration(rand.Int64N(int64(restoreJitter)))); err != nil {
				m.restoreAborted(instance, err)
				return
			}
			if err := m.restore(ctx, instance); err != nil {
				m.log.Warn().Str("instance_id", instance.ID.String()).Err(err).Msg("restore session failed")
				m.emitConnection(instance.ID, session.StatusError, instance.Connection.DeviceJID, err.Error())
			} else if sess, ok := m.Get(instance.ID); ok && sess != nil {
				m.log.Info().Str("instance_id", instance.ID.String()).Str("jid", instance.Connection.DeviceJID).
					Str("live_status", string(sess.Status())).Bool("socket_connected", sess.IsConnected()).
					Msg("restore session dialed")
			} else {
				m.log.Warn().Str("instance_id", instance.ID.String()).Msg("restore session missing after success")
			}
		}(instance)
	}
	wg.Wait()
	return nil
}

// restoreAborted reports a restore that the context ended before it could run,
// so the instance status reflects that it was not restored.
func (m *Manager) restoreAborted(instance model.Instance, err error) {
	reason := "restore cancelled: " + err.Error()
	m.log.Warn().Str("instance_id", instance.ID.String()).Err(err).Msg("restore session cancelled")
	m.emitConnection(instance.ID, session.StatusError, instance.Connection.DeviceJID, reason)
}

// restore attaches the persisted device of instance and brings it online.
func (m *Manager) restore(ctx context.Context, instance model.Instance) error {
	device, err := m.loadBoundDevice(ctx, &instance)
	if err != nil {
		m.log.Warn().Str("instance_id", instance.ID.String()).Str("jid", instance.BoundDeviceJID()).Err(err).Msg("restore load device failed")
		return err
	}
	m.log.Debug().Str("instance_id", instance.ID.String()).Str("jid", instance.BoundDeviceJID()).Bool("deleted", device.ID == nil).Msg("restore device loaded")
	return m.attachAndConnect(ctx, instance.ID, device)
}

// attachAndConnect registers a session built from device and brings it online.
// A concurrent Create or RestoreAll that wins the registration turns this into
// a no-op, so a client that is not in the manager is never connected: two live
// clients on the same device would fight over the session.
func (m *Manager) attachAndConnect(ctx context.Context, instanceID uuid.UUID, device *store.Device) error {
	sess, err := newSession(instanceID, device, m.log, m.sink, m.maxMediaBytes)
	if err != nil {
		m.log.Warn().Str("instance_id", instanceID.String()).Err(err).Msg("attach new session failed")
		return err
	}
	if !m.registerRestored(instanceID, sess) {
		m.log.Debug().Str("instance_id", instanceID.String()).Msg("attach skipped: already registered")
		return nil
	}
	connect := m.restoreConnect
	if connect == nil {
		connect = func(ctx context.Context, sess *instanceSession) error {
			return sess.connectExisting(ctx)
		}
	}
	// A transient handshake failure at startup must not park the instance
	// in error forever: the session is already registered with its
	// credentials, so arm the backoff loop and report disconnected
	// (retrying) instead of an error that needs attention.
	if err := connect(ctx, sess); err != nil {
		m.log.Warn().Str("instance_id", instanceID.String()).Err(err).
			Str("live_status", string(sess.Status())).Bool("socket_connected", sess.IsConnected()).
			Msg("attach connectExisting failed")
		if retryableRestore(err) {
			sess.scheduleRestoreRetry(err)
			return nil
		}
		return err
	}
	m.log.Debug().Str("instance_id", instanceID.String()).
		Str("live_status", string(sess.Status())).Bool("socket_connected", sess.IsConnected()).
		Msg("attach connectExisting dialed")
	return nil
}

// registerRestored stores sess for instanceID unless another goroutine already
// registered a session, reporting whether this call performed the insert.
func (m *Manager) registerRestored(instanceID uuid.UUID, sess *instanceSession) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[instanceID]; ok {
		return false
	}
	m.sessions[instanceID] = sess
	return true
}

// listInstances reads the complete persisted instance collection.
func (m *Manager) listInstances(ctx context.Context) ([]model.Instance, error) {
	instances, err := m.instances.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	return instances, nil
}

// deviceFor returns the device store of instance, creating a fresh one when
// the instance was never paired.
func (m *Manager) deviceFor(ctx context.Context, instance *model.Instance) (*store.Device, error) {
	bound := instance.BoundDeviceJID()
	if bound == "" {
		return m.devices.NewDevice(), nil
	}
	return m.loadBoundDevice(ctx, instance)
}

// loadBoundDevice loads the whatsmeow store for the device JID bound to
// instance, rejecting cross-instance reuse and store mismatches.
func (m *Manager) loadBoundDevice(ctx context.Context, instance *model.Instance) (*store.Device, error) {
	bound := instance.BoundDeviceJID()
	if bound == "" {
		return nil, fmt.Errorf("load bound device: empty jid")
	}
	if m.instances != nil {
		owner, err := m.instances.GetByDeviceJID(ctx, bound)
		if err == nil && owner.ID != instance.ID {
			return nil, fmt.Errorf("create session: device jid already bound to instance %s: %w", owner.ID, session.ErrDeviceJIDTaken)
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("create session: lookup device jid: %w", err)
		}
	}
	jid, err := types.ParseJID(bound)
	if err != nil {
		return nil, fmt.Errorf("create session: parse jid %q: %w", bound, err)
	}
	device, err := m.devices.GetDevice(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("create session: load device %s: %w", jid, err)
	}
	if device == nil {
		return nil, fmt.Errorf("create session: device %s: %w", jid, session.ErrNoDevice)
	}
	if device.ID != nil && device.ID.String() != bound {
		return nil, fmt.Errorf("create session: device jid mismatch (store %s, instance %s): %w", device.ID, bound, session.ErrNoDevice)
	}
	return device, nil
}

// emitConnection reports a connection change for an instance with no session
// (used when restoring fails before a session exists).
func (m *Manager) emitConnection(instanceID uuid.UUID, status session.Status, jid, reason string) {
	if m.sink == nil {
		return
	}
	m.sink.OnConnection(context.Background(), instanceID, status, jid, reason)
}

// sleepCtx waits for d unless the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// instanceSession is the session.Session implementation for one instance.
type instanceSession struct {
	instanceID    uuid.UUID
	client        *whatsmeow.Client
	sink          session.EventSink
	log           zerolog.Logger
	maxMediaBytes int64

	// sleep and backoff drive the auto-reconnect; tests replace them to assert
	// the retry transitions without real waits.
	sleep   sleepFunc
	backoff reconnectPolicy
	// reconnectFn brings the session online during an auto-reconnect. It
	// defaults to connectExisting and is replaced in the transition tests.
	reconnectFn func(ctx context.Context) error
	// getQRChannelFn and connectFn isolate the two whatsmeow boundaries used
	// while pairing. Tests replace them to verify the connection context
	// without opening a real WhatsApp websocket.
	getQRChannelFn func(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error)
	connectFn      func(ctx context.Context) error
	// logoutFn removes the companion device from WhatsApp. It defaults to the
	// client Logout and is replaced in the tests to assert the attempt without
	// a network handshake.
	logoutFn func(ctx context.Context) error
	// sendRevokeFn sends a revoke protocol message. It defaults to SendMessage
	// and is replaced in the tests to assert the built revocation without a
	// network round-trip.
	sendRevokeFn func(ctx context.Context, chat types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error)
	// markReadFn sends a read receipt. It defaults to the client MarkRead and
	// is replaced in the tests to assert the receipt arguments.
	markReadFn func(ctx context.Context, ids []types.MessageID, timestamp time.Time, chat, sender types.JID) error
	// pairPhoneFn requests a phone pairing code. It defaults to the client
	// PairPhone and is replaced in the tests to avoid the pairing handshake.
	pairPhoneFn func(ctx context.Context, phone string) (string, error)
	// decryptVoteFn decrypts the selected option hashes of an inbound poll
	// vote. It defaults to the client DecryptPollVote and is replaced in the
	// tests to avoid the message-secret handshake.
	decryptVoteFn func(ctx context.Context, evt *events.Message) ([][]byte, error)
	// statusSendFn publishes a status message to the status broadcast. It
	// defaults to the client SendMessage and is replaced in the tests to
	// assert the built status without a network round-trip.
	statusSendFn func(ctx context.Context, msg *waE2E.Message) (whatsmeow.SendResponse, error)
	// statusUploadFn uploads status media bytes. It defaults to the client
	// Upload and is replaced in the tests to avoid the media handshake.
	statusUploadFn func(ctx context.Context, data []byte, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error)
	// setGroupPhotoFn replaces the group picture upstream. It defaults to the
	// client SetGroupPhoto and is replaced in tests to assert classification.
	setGroupPhotoFn func(ctx context.Context, jid types.JID, image []byte) (string, error)
	// getUserInfoFn fetches user metadata. It defaults to the client GetUserInfo
	// and is replaced in profile tests.
	getUserInfoFn func(ctx context.Context, jids []types.JID) (map[types.JID]types.UserInfo, error)
	// getProfilePictureInfoFn fetches a profile picture. It defaults to the
	// client GetProfilePictureInfo and is replaced in profile tests.
	getProfilePictureInfoFn func(ctx context.Context, jid types.JID, params *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error)
	// isConnectedFn reports socket liveness. It defaults to the client and is
	// replaced in tests that exercise connected-only session methods.
	isConnectedFn func() bool

	// history accumulates the per-instance history-sync feed the Import plan
	// consumes. Each session owns one, so feeds never cross instance
	// boundaries. The zero value is ready to use.
	history session.HistorySyncAccumulator
	// statusMu guards statuses, the process-local registry of the own
	// statuses published through this session. The supported runtime is one
	// replica, so no shared store is needed.
	statusMu sync.Mutex
	statuses []session.StatusInfo
	// disappearingMu guards disappearing, the process-local record of the
	// timers set through this session. The pinned library exposes no fetch,
	// so GetDisappearingTimer only knows what this session set.
	disappearingMu sync.RWMutex
	disappearing   map[string]time.Duration

	mu       sync.RWMutex
	status   session.Status
	jid      string
	paired   bool
	terminal bool
	// dialed reports that a connect attempt opened the socket without the
	// login completing yet: ConnectContext returned nil but no Connected
	// event arrived since. A drop in this window means WhatsApp rejected
	// the session (stale/duplicate device), not a transient network blip.
	dialed       bool
	qrCode       string
	qrExpiresAt  time.Time
	firstQR      chan qrResult
	qrCancel     context.CancelFunc
	lastReason   string
	reconnectRun *reconnectRun
}

var _ session.Session = (*instanceSession)(nil)

// qrResult carries the outcome of a pairing attempt to the caller waiting in
// Connect.
type qrResult struct {
	code      string
	expiresAt time.Time
	err       error
}

// newSession wraps device in a connected-aware session and registers the event
// translation. maxMediaBytes caps how much inbound media a download may
// buffer.
func newSession(instanceID uuid.UUID, device *store.Device, log zerolog.Logger, sink session.EventSink, maxMediaBytes int64) (*instanceSession, error) {
	if device == nil {
		return nil, errors.New("new session: nil device")
	}
	sess := &instanceSession{
		instanceID:    instanceID,
		sink:          sink,
		log:           log,
		maxMediaBytes: maxMediaBytes,
		status:        session.StatusDisconnected,
		sleep:         sleepCtx,
		backoff: reconnectPolicy{
			base:   reconnectBaseDelay,
			max:    reconnectMaxDelay,
			jitter: defaultJitter,
		},
	}
	client := whatsmeow.NewClient(device, newWALogger(log))
	// wzap drives its own exponential backoff; the library reconnect would
	// otherwise race it with a linear, uncapped schedule.
	client.EnableAutoReconnect = false
	if device.ID != nil && !device.ID.IsEmpty() {
		sess.jid = device.ID.String()
		sess.paired = true
	}
	client.AddEventHandler(sess.dispatch)
	sess.client = client
	sess.reconnectFn = sess.connectExisting
	return sess, nil
}

// Status returns the current lifecycle state.
func (s *instanceSession) Status() session.Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// JID returns the public JID, empty while the instance is not paired.
func (s *instanceSession) JID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jid
}

// IsConnected reports whether the WhatsApp websocket is currently live. It
// is the live-socket side of the health check: Status (and the DB-persisted
// instances.status fed by the connection events) can say "connected" while
// the socket is already dead, so callers must consult both.
func (s *instanceSession) IsConnected() bool {
	if s == nil || s.client == nil {
		return false
	}
	if s.isConnectedFn != nil {
		return s.isConnectedFn()
	}
	return s.client.IsConnected()
}

// setStatus updates the lifecycle state, reports it to the sink when it
// changed and applies the reconnect policy of the transition. A paired JID
// marks the session as reconnectable, so a connected event after a pairing
// turns later drops into retries. Terminal states (error and every
// disconnected transition carrying a reason) stick: the socket close that
// follows a restriction or a logout must not turn it into a retry.
//
// An immediate post-connect drop (a bare Disconnected while a dial is
// pending, i.e. the socket opened but no Connected ever arrived) is the
// rejected-session symptom, not a transient blip: it becomes a terminal
// error with an actionable reason instead of an infinite retry (or a silent
// no-op when the session never left disconnected).
func (s *instanceSession) setStatus(status session.Status, jid, reason string) {
	s.mu.Lock()
	if status == session.StatusDisconnected && reason == "" && s.terminal {
		s.mu.Unlock()
		return
	}
	if status == session.StatusDisconnected && reason == "" && s.dialed {
		status = session.StatusError
		reason = session.SessionRejectedReason
	}
	if s.status == status && s.lastReason == reason {
		s.mu.Unlock()
		return
	}
	from := s.status
	s.status = status
	s.lastReason = reason
	s.terminal = status == session.StatusError || (status == session.StatusDisconnected && reason != "")
	if status == session.StatusConnected || s.terminal {
		s.dialed = false
	}
	if jid != "" {
		s.jid = jid
		s.paired = true
	}
	currentJID := s.jid
	s.mu.Unlock()

	s.log.Debug().Str("instance_id", s.instanceID.String()).Str("from", string(from)).Str("to", string(status)).Bool("jid_present", currentJID != "").Str("reason", reason).Bool("socket_connected", s.client.IsConnected()).Msg("session status changed")
	if status == session.StatusError {
		s.log.Warn().Str("instance_id", s.instanceID.String()).Str("from", string(from)).Str("reason", reason).Msg("session entered error status")
	} else if status == session.StatusDisconnected && from == session.StatusConnected {
		s.log.Info().Str("instance_id", s.instanceID.String()).Str("from", string(from)).Str("to", string(status)).Bool("socket_connected", s.client.IsConnected()).Msg("websocket disconnected, retrying with backoff")
	}

	if s.sink != nil {
		s.sink.OnConnection(context.Background(), s.instanceID, status, currentJID, reason)
	}
	s.applyConnectionPolicy(status, reason)
}

// markDialed records that a connect attempt opened the socket and the login
// is now pending: the next bare Disconnected means the session was rejected.
// It is set on every path where ConnectContext succeeds for an already
// paired device (connectExisting and the stored-credentials branch of
// Connect), and cleared by the Connected event or by any terminal state.
func (s *instanceSession) markDialed() {
	s.mu.Lock()
	s.dialed = true
	s.mu.Unlock()
}

// Send delivers an outbound message through the whatsmeow client.
func (s *instanceSession) Send(ctx context.Context, msg session.OutboundMessage) (string, error) {
	if !s.client.IsConnected() {
		return "", fmt.Errorf("%w: send %s", session.ErrNotConnected, msg.Type)
	}
	recipient, err := types.ParseJID(msg.RecipientJID)
	if err != nil {
		return "", fmt.Errorf("%w: %s", session.ErrInvalidRecipient, msg.RecipientJID)
	}

	var waMsg *waE2E.Message
	if isMediaType(msg.Type) {
		waMsg, err = s.uploadMedia(ctx, msg)
	} else {
		waMsg, err = buildMessage(msg)
	}
	if err != nil {
		return "", err
	}

	resp, err := s.client.SendMessage(ctx, recipient, waMsg)
	if err != nil {
		return "", classifySessionError(err)
	}
	return string(resp.ID), nil
}

// IsOnWhatsApp resolves a phone number to its canonical JID.
func (s *instanceSession) IsOnWhatsApp(ctx context.Context, phone string) (string, bool, error) {
	if !s.client.IsConnected() {
		// O Status pode dizer "connected" com o socket já morto (o banco só
		// vê o drop quando o evento chega): registra ambos no momento da
		// falha para expor a divergência.
		s.log.Warn().Str("instance_id", s.instanceID.String()).Str("live_status", string(s.Status())).Bool("socket_connected", false).Msg("isOnWhatsApp blocked: socket down")
		return "", false, fmt.Errorf("%w: is on whatsapp", session.ErrNotConnected)
	}
	responses, err := s.client.IsOnWhatsApp(ctx, []string{phone})
	if err != nil {
		return "", false, classifySessionError(err)
	}
	if len(responses) == 0 || !responses[0].IsIn || responses[0].JID.IsEmpty() {
		return "", false, nil
	}
	return responses[0].JID.String(), true, nil
}

// presenceRequest is a parsed SendPresence state.
type presenceRequest struct {
	isChat bool
	chat   types.ChatPresence
	user   types.Presence
}

// parsePresence classifies the presence states accepted by SendPresence.
func parsePresence(state string) (presenceRequest, error) {
	switch types.ChatPresence(state) {
	case types.ChatPresenceComposing, types.ChatPresencePaused:
		return presenceRequest{isChat: true, chat: types.ChatPresence(state)}, nil
	}
	switch types.Presence(state) {
	case types.PresenceAvailable, types.PresenceUnavailable:
		return presenceRequest{user: types.Presence(state)}, nil
	}
	return presenceRequest{}, fmt.Errorf("unsupported presence state %q", state)
}

// SendPresence reports chat typing or user availability.
func (s *instanceSession) SendPresence(ctx context.Context, chatJID, state string) error {
	target, err := parsePresence(state)
	if err != nil {
		return err
	}
	jid, err := types.ParseJID(chatJID)
	if err != nil {
		return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, chatJID)
	}
	if target.isChat {
		return s.client.SendChatPresence(ctx, jid, target.chat, types.ChatPresenceMediaText)
	}
	return s.client.SendPresence(ctx, target.user)
}

// DeleteMessage revokes a sent message for everyone in the chat. The pinned
// library deprecated RevokeMessage in favor of BuildRevoke + SendMessage, so
// the session builds the REVOKE protocol message keyed by the original id and
// sends it to the chat. Connectivity failures surface through the shared
// classifier (not-connected vs transient).
func (s *instanceSession) DeleteMessage(ctx context.Context, chatJID, messageID string) error {
	// ParseJID in the pinned library is lenient (it only splits user/server),
	// so an empty result is rejected explicitly; anything else parses like
	// Send does.
	chat, err := types.ParseJID(chatJID)
	if err != nil || chat.IsEmpty() {
		return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, chatJID)
	}
	if messageID == "" {
		return errors.New("delete message: empty message id")
	}
	send := s.sendRevokeFn
	if send == nil {
		send = func(ctx context.Context, chat types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
			return s.client.SendMessage(ctx, chat, msg)
		}
	}
	if _, err := send(ctx, chat, s.client.BuildRevoke(chat, types.EmptyJID, types.MessageID(messageID))); err != nil {
		return classifySessionError(err)
	}
	return nil
}

// MarkRead sends a read receipt for one message. An empty senderJID falls
// back to the chat JID (direct chats); group reads carry the author, as the
// library requires. The receipt is stamped at read time.
func (s *instanceSession) MarkRead(ctx context.Context, chatJID, senderJID, messageID string) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil || chat.IsEmpty() {
		return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, chatJID)
	}
	sender := chat
	if senderJID != "" {
		sender, err = types.ParseJID(senderJID)
		if err != nil || sender.IsEmpty() {
			return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, senderJID)
		}
	}
	if messageID == "" {
		return errors.New("mark read: empty message id")
	}
	mark := s.markReadFn
	if mark == nil {
		mark = func(ctx context.Context, ids []types.MessageID, timestamp time.Time, chat, sender types.JID) error {
			return s.client.MarkRead(ctx, ids, timestamp, chat, sender)
		}
	}
	if err := mark(ctx, []types.MessageID{types.MessageID(messageID)}, time.Now().UTC(), chat, sender); err != nil {
		return classifySessionError(err)
	}
	return nil
}

// Disconnect asks WhatsApp to drop the companion device while the connection
// is still up, then closes it, stops any pending auto-reconnect and clears the
// paired identity of the session. A logout the server cannot be reached for is
// not fatal: the local teardown still completes.
func (s *instanceSession) Disconnect(ctx context.Context) error {
	s.cancelQR()
	s.cancelReconnect()
	s.logoutTolerantly(ctx)
	s.client.Disconnect()

	s.mu.Lock()
	s.jid = ""
	s.paired = false
	s.mu.Unlock()

	s.setStatus(session.StatusDisconnected, "", "disconnected by request")
	return nil
}

// remove asks WhatsApp to drop the companion device, disconnects the client and
// deletes its stored credentials. It is safe to call it after Disconnect, when
// the logout attempt only sees a dropped connection.
func (s *instanceSession) remove(ctx context.Context) error {
	s.cancelQR()
	s.cancelReconnect()
	s.logoutTolerantly(ctx)
	s.client.Disconnect()
	if s.client.Store.ID != nil {
		if err := s.client.Store.Delete(ctx); err != nil {
			return fmt.Errorf("delete device: %w", err)
		}
	}
	// A session already disconnected through the API emitted its own event;
	// only report the removal when it actually changed the state.
	if s.Status() != session.StatusDisconnected {
		s.setStatus(session.StatusDisconnected, "", "session removed")
	}
	return nil
}

// logout asks WhatsApp to remove the companion device, defaulting to the
// client Logout. Tests replace logoutFn to assert the attempt.
func (s *instanceSession) logout(ctx context.Context) error {
	if s.logoutFn != nil {
		return s.logoutFn(ctx)
	}
	if s.client == nil {
		return nil
	}
	return s.client.Logout(ctx)
}

// logoutTolerantly attempts the WhatsApp logout. A session that was never
// online and a device that is already gone are treated as done: the local
// teardown continues in every case and only unexpected failures are logged.
func (s *instanceSession) logoutTolerantly(ctx context.Context) {
	err := s.logout(ctx)
	switch {
	case err == nil,
		errors.Is(err, whatsmeow.ErrNotConnected),
		errors.Is(err, whatsmeow.ErrNotLoggedIn),
		errors.Is(err, store.ErrDeviceDeleted):
		return
	}
	s.log.Warn().Str("instance_id", s.instanceID.String()).Err(err).Msg("logout from whatsapp failed")
}

// connectExisting brings an already paired device online. A nil return only
// means the socket opened: the login still has to complete with a Connected
// event, so the dial is marked pending for the post-connect drop detection.
func (s *instanceSession) connectExisting(ctx context.Context) error {
	s.log.Debug().Str("instance_id", s.instanceID.String()).
		Bool("socket_connected", s.client.IsConnected()).Str("status", string(s.Status())).
		Msg("connecting existing session")
	if err := s.client.ConnectContext(ctx); err != nil {
		if errors.Is(err, whatsmeow.ErrAlreadyConnected) {
			s.log.Debug().Str("instance_id", s.instanceID.String()).Msg("session already connected")
			return nil
		}
		s.log.Warn().Str("instance_id", s.instanceID.String()).Err(err).
			Bool("socket_connected", s.client.IsConnected()).Str("live_status", string(s.Status())).
			Msg("connect existing session failed")
		return classifySessionError(err)
	}
	s.markDialed()
	s.log.Debug().Str("instance_id", s.instanceID.String()).
		Bool("socket_connected", s.client.IsConnected()).Str("live_status", string(s.Status())).
		Msg("connect existing session dialed, waiting for login")
	return nil
}

// cancelQR stops the pairing channel of the session, when one is open.
func (s *instanceSession) cancelQR() {
	s.mu.Lock()
	cancel := s.qrCancel
	s.qrCancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// classifySessionError maps a whatsmeow error onto the errors callers act on.
func classifySessionError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, whatsmeow.ErrNotConnected) || errors.Is(err, whatsmeow.ErrNotLoggedIn) {
		return fmt.Errorf("%w: %v", session.ErrNotConnected, err)
	}
	return fmt.Errorf("%w: %v", session.ErrTransient, err)
}

// outboundPayload is the JSON body shared by every message type. QuotedID is
// the WhatsApp id being replied to, empty when the message is not a quote.
// The rich fields carry the stored poll, reaction, list and buttons bodies:
// Target is the WhatsApp id of the reacted message (an empty Emoji removes
// the reaction), and the list/buttons sections mirror the API shapes.
type outboundPayload struct {
	Text        string   `json:"text"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	DisplayName string   `json:"display_name"`
	VCard       string   `json:"vcard"`
	Caption     string   `json:"caption"`
	Filename    string   `json:"filename"`
	MimeType    string   `json:"mime_type"`
	PTT         bool     `json:"ptt"`
	QuotedID    string   `json:"quoted_id"`

	Question        string            `json:"question"`
	Options         []string          `json:"options"`
	SelectableCount int               `json:"selectable_count"`
	Target          string            `json:"target"`
	Emoji           string            `json:"emoji"`
	Title           string            `json:"title"`
	Description     string            `json:"description"`
	ButtonText      string            `json:"button_text"`
	Sections        []outboundSection `json:"sections"`
	Footer          string            `json:"footer"`
	Buttons         []outboundButton  `json:"buttons"`
}

// outboundSection is one section of a list message payload.
type outboundSection struct {
	Title string        `json:"title"`
	Rows  []outboundRow `json:"rows"`
}

// outboundRow is one selectable row of a list section payload.
type outboundRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// outboundButton is one quick-reply button of a buttons message payload.
type outboundButton struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// buildMessage builds a non-media message from its normalized payload.
func buildMessage(msg session.OutboundMessage) (*waE2E.Message, error) {
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return nil, err
	}
	switch msg.Type {
	case "text":
		if payload.Text == "" {
			return nil, errors.New("text payload is empty")
		}
		// A plain conversation cannot carry a reply stanza, so quoted text
		// goes as an extended text message with the quote attached.
		if payload.QuotedID != "" {
			return &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        proto.String(payload.Text),
				ContextInfo: quotedContext(payload.QuotedID),
			}}, nil
		}
		return &waE2E.Message{Conversation: proto.String(payload.Text)}, nil
	case "location":
		if payload.Latitude == nil || payload.Longitude == nil {
			return nil, errors.New("location payload is missing coordinates")
		}
		return &waE2E.Message{LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  payload.Latitude,
			DegreesLongitude: payload.Longitude,
			Name:             optionalString(payload.Name),
			Address:          optionalString(payload.Address),
		}}, nil
	case "contact":
		if payload.VCard == "" {
			return nil, errors.New("contact payload is missing the vcard")
		}
		return &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
			DisplayName: optionalString(payload.DisplayName),
			Vcard:       proto.String(payload.VCard),
		}}, nil
	case "poll":
		return buildPoll(payload)
	case "reaction":
		return buildReaction(msg.RecipientJID, payload)
	case "list":
		return buildList(payload)
	case "buttons":
		return buildButtons(payload)
	}
	return nil, fmt.Errorf("unsupported message type %q", msg.Type)
}

// buildPoll builds a poll creation message from its normalized payload. The
// shape mirrors the library BuildPollCreation (name, ordered options,
// selectable count) with a fresh random message secret.
func buildPoll(payload outboundPayload) (*waE2E.Message, error) {
	if payload.Question == "" {
		return nil, errors.New("poll payload is empty")
	}
	if len(payload.Options) < 2 {
		return nil, errors.New("poll payload needs at least two options")
	}
	options := make([]*waE2E.PollCreationMessage_Option, len(payload.Options))
	for i, option := range payload.Options {
		options[i] = &waE2E.PollCreationMessage_Option{OptionName: proto.String(option)}
	}
	secret := make([]byte, 32)
	if _, err := cryptorand.Read(secret); err != nil {
		return nil, fmt.Errorf("poll secret: %w", err)
	}
	return &waE2E.Message{
		PollCreationMessage: &waE2E.PollCreationMessage{
			Name:                   proto.String(payload.Question),
			Options:                options,
			SelectableOptionsCount: proto.Uint32(uint32(payload.SelectableCount)),
		},
		MessageContextInfo: &waE2E.MessageContextInfo{
			MessageSecret: secret,
		},
	}, nil
}

// buildReaction builds a reaction to a message sent by the instance. The key
// marks the reacted message as from-me keyed by the chat: reactions to
// messages sent by other participants are out of scope. An empty emoji
// removes the reaction on the same target, on the same endpoint.
func buildReaction(chatJID string, payload outboundPayload) (*waE2E.Message, error) {
	if payload.Target == "" {
		return nil, errors.New("reaction payload is missing the target")
	}
	chat, err := types.ParseJID(chatJID)
	if err != nil || chat.IsEmpty() {
		return nil, fmt.Errorf("%w: %s", session.ErrInvalidRecipient, chatJID)
	}
	return &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key: &waCommon.MessageKey{
				FromMe:    proto.Bool(true),
				ID:        proto.String(payload.Target),
				RemoteJID: proto.String(chat.String()),
			},
			Text:              proto.String(payload.Emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}, nil
}

// buildList builds an interactive single-select list message from its
// normalized payload.
func buildList(payload outboundPayload) (*waE2E.Message, error) {
	if payload.ButtonText == "" {
		return nil, errors.New("list payload is missing the button text")
	}
	if len(payload.Sections) == 0 {
		return nil, errors.New("list payload needs at least one section")
	}
	sections := make([]*waE2E.ListMessage_Section, 0, len(payload.Sections))
	for _, section := range payload.Sections {
		if len(section.Rows) == 0 {
			return nil, errors.New("list payload needs at least one row per section")
		}
		rows := make([]*waE2E.ListMessage_Row, 0, len(section.Rows))
		for _, row := range section.Rows {
			rows = append(rows, &waE2E.ListMessage_Row{
				RowID:       proto.String(row.ID),
				Title:       proto.String(row.Title),
				Description: optionalString(row.Description),
			})
		}
		sections = append(sections, &waE2E.ListMessage_Section{
			Title: proto.String(section.Title),
			Rows:  rows,
		})
	}
	return &waE2E.Message{ListMessage: &waE2E.ListMessage{
		Title:       optionalString(payload.Title),
		Description: optionalString(payload.Description),
		ButtonText:  proto.String(payload.ButtonText),
		ListType:    waE2E.ListMessage_SINGLE_SELECT.Enum(),
		Sections:    sections,
		FooterText:  optionalString(payload.Footer),
	}}, nil
}

// buildButtons builds an interactive quick-reply buttons message from its
// normalized payload. A PIX key travels as button content pass-through (for
// example in the title): there is no distinct PIX button type.
func buildButtons(payload outboundPayload) (*waE2E.Message, error) {
	if payload.Text == "" {
		return nil, errors.New("buttons payload is empty")
	}
	if len(payload.Buttons) == 0 {
		return nil, errors.New("buttons payload needs at least one button")
	}
	buttons := make([]*waE2E.ButtonsMessage_Button, 0, len(payload.Buttons))
	for _, button := range payload.Buttons {
		buttons = append(buttons, &waE2E.ButtonsMessage_Button{
			ButtonID: proto.String(button.ID),
			ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{
				DisplayText: proto.String(button.Title),
			},
			Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
		})
	}
	return &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{
		ContentText: proto.String(payload.Text),
		FooterText:  optionalString(payload.Footer),
		Buttons:     buttons,
	}}, nil
}

// newMediaMessage builds a media message from an uploaded attachment.
func newMediaMessage(msg session.OutboundMessage, upload whatsmeow.UploadResponse) (*waE2E.Message, error) {
	if !isMediaType(msg.Type) {
		return nil, fmt.Errorf("unsupported media type %q", msg.Type)
	}
	payload, err := decodePayload(msg.Payload)
	if err != nil {
		return nil, err
	}
	if payload.MimeType == "" {
		return nil, errors.New("media payload is missing the mime type")
	}
	var ctxInfo *waE2E.ContextInfo
	if payload.QuotedID != "" {
		ctxInfo = quotedContext(payload.QuotedID)
	}
	switch msg.Type {
	case "sticker":
		if payload.MimeType != "image/webp" {
			return nil, fmt.Errorf("unsupported sticker mimetype %q", payload.MimeType)
		}
		return &waE2E.Message{StickerMessage: &waE2E.StickerMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileSHA256:    upload.FileSHA256,
			FileEncSHA256: upload.FileEncSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(payload.MimeType),
			ContextInfo:   ctxInfo,
		}}, nil
	case "image":
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileSHA256:    upload.FileSHA256,
			FileEncSHA256: upload.FileEncSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(payload.MimeType),
			Caption:       optionalString(payload.Caption),
			ContextInfo:   ctxInfo,
		}}, nil
	case "video":
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileSHA256:    upload.FileSHA256,
			FileEncSHA256: upload.FileEncSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(payload.MimeType),
			Caption:       optionalString(payload.Caption),
			ContextInfo:   ctxInfo,
		}}, nil
	case "audio":
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileSHA256:    upload.FileSHA256,
			FileEncSHA256: upload.FileEncSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(payload.MimeType),
			PTT:           proto.Bool(payload.PTT),
			ContextInfo:   ctxInfo,
		}}, nil
	case "document":
		return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileSHA256:    upload.FileSHA256,
			FileEncSHA256: upload.FileEncSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(payload.MimeType),
			Caption:       optionalString(payload.Caption),
			FileName:      optionalString(payload.Filename),
			Title:         optionalString(payload.Filename),
			ContextInfo:   ctxInfo,
		}}, nil
	}
	return nil, fmt.Errorf("unsupported media type %q", msg.Type)
}

// uploadMedia reads the file of a media message and uploads it to WhatsApp.
func (s *instanceSession) uploadMedia(ctx context.Context, msg session.OutboundMessage) (*waE2E.Message, error) {
	data, err := os.ReadFile(msg.MediaPath)
	if err != nil {
		return nil, fmt.Errorf("read media %s: %w", msg.MediaPath, err)
	}
	upload, err := s.client.Upload(ctx, data, mediaTypeFor(msg.Type))
	if err != nil {
		return nil, classifySessionError(err)
	}
	return newMediaMessage(msg, upload)
}

// decodePayload parses the JSON body of an outbound message. An empty payload
// decodes to its zero value.
func decodePayload(raw []byte) (outboundPayload, error) {
	var payload outboundPayload
	if len(raw) == 0 {
		return payload, nil
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return outboundPayload{}, fmt.Errorf("decode payload: %w", err)
	}
	return payload, nil
}

// isMediaType reports whether type is delivered as an uploaded attachment,
// including stickers (webp uploaded under the image namespace).
func isMediaType(messageType string) bool {
	switch messageType {
	case "image", "video", "audio", "document", "sticker":
		return true
	}
	return false
}

// mediaTypeFor maps a message type to the whatsmeow media key namespace.
// Stickers upload under the image namespace, like the library does.
func mediaTypeFor(messageType string) whatsmeow.MediaType {
	switch messageType {
	case "image", "sticker":
		return whatsmeow.MediaImage
	case "video":
		return whatsmeow.MediaVideo
	case "audio":
		return whatsmeow.MediaAudio
	case "document":
		return whatsmeow.MediaDocument
	}
	return ""
}

// quotedContext builds the reply stanza keyed by the original WhatsApp
// message id. The quoted content is not stored locally, so the stanza carries
// an empty placeholder message and clients resolve the original by stanza id
// on a best-effort basis.
func quotedContext(quotedID string) *waE2E.ContextInfo {
	return &waE2E.ContextInfo{
		StanzaID:      proto.String(quotedID),
		QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
	}
}

// optionalString returns a proto string pointer, or nil for an empty value.
func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return proto.String(value)
}
