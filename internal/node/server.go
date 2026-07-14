package node

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/coder/websocket"

	"phosphornet/internal/action"
	"phosphornet/internal/config"
	"phosphornet/internal/identity"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
	"phosphornet/internal/storage"
)

const defaultSessionDisconnectGrace = 5 * time.Second

type Server struct {
	cfg             config.NodeConfig
	doorsMu         sync.RWMutex
	doors           []runtime.DoorManifest
	store           *storage.Store
	sessions        *sessionRegistry
	events          *eventLog
	auditLog        *auditSink
	auditMaxBytes   int64
	rateLimits      *userRateTracker
	disconnectGrace time.Duration
	actionExecutor  action.Executor
}

func newServer(cfg config.NodeConfig, doors []runtime.DoorManifest, store *storage.Store) *Server {
	return newServerWithOptions(cfg, doors, store, serverOptions{})
}

func newServerWithOptions(cfg config.NodeConfig, doors []runtime.DoorManifest, store *storage.Store, options serverOptions) *Server {
	disconnectGrace := options.DisconnectGrace
	if disconnectGrace == 0 {
		disconnectGrace = defaultSessionDisconnectGrace
	}
	executor := options.ActionExecutor
	if executor == nil && cfg.Actiond.Enabled && cfg.Actiond.Socket != "" {
		executor = action.Client{Socket: cfg.Actiond.Socket}
	}
	return &Server{
		cfg:             cfg,
		doors:           doors,
		store:           store,
		sessions:        newSessionRegistry(),
		events:          newEventLog(100),
		auditLog:        &auditSink{file: options.AuditLogFile},
		auditMaxBytes:   options.AuditLogMaxBytes,
		rateLimits:      newUserRateTracker(),
		disconnectGrace: disconnectGrace,
		actionExecutor:  executor,
	}
}

func (s *Server) doorManifests() []runtime.DoorManifest {
	s.doorsMu.RLock()
	defer s.doorsMu.RUnlock()

	doors := make([]runtime.DoorManifest, len(s.doors))
	copy(doors, s.doors)
	return doors
}

func (s *Server) setDoorManifests(doors []runtime.DoorManifest) {
	s.doorsMu.Lock()
	defer s.doorsMu.Unlock()

	s.doors = make([]runtime.DoorManifest, len(doors))
	copy(s.doors, doors)
}

type eventLog struct {
	mu     sync.Mutex
	limit  int
	events []protocol.RuntimeEvent
}

func newEventLog(limit int) *eventLog {
	return &eventLog{limit: limit}
}

func (l *eventLog) add(eventType, doorID, publicKey, message string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	event := protocol.RuntimeEvent{
		Time:        time.Now().UTC().Format(time.RFC3339),
		Type:        eventType,
		DoorID:      doorID,
		Fingerprint: fingerprintForPublicKey(publicKey),
		Message:     message,
	}
	l.events = append(l.events, event)
	if l.limit > 0 && len(l.events) > l.limit {
		l.events = append([]protocol.RuntimeEvent{}, l.events[len(l.events)-l.limit:]...)
	}
}

func (l *eventLog) recent() []protocol.RuntimeEvent {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	events := make([]protocol.RuntimeEvent, len(l.events))
	copy(events, l.events)
	return events
}

func (l *eventLog) clear() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = nil
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(protocol.MaxWebSocketMessageBytes)

	ctx := context.Background()
	authCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	session, ok := s.authenticate(authCtx, conn)
	cancel()
	if !ok {
		return
	}
	pending := s.sessions.claimPendingReconnect(session.publicKey)
	session.conn = conn
	s.sessions.add(session)
	defer func() {
		s.beginSessionDisconnect(session)
	}()

	if err := s.sendDoorList(ctx, session); err != nil {
		return
	}
	initialDoor, suppressJoin := s.initialDoorForReconnect(ctx, session, pending)
	if err := s.openInitialDoor(ctx, session, initialDoor, suppressJoin); err != nil {
		_ = session.write(ctx, protocol.NotifyMessage{
			Type:    protocol.TypeNotify,
			Level:   "warn",
			Message: fmt.Sprintf("%s door failed to render: %v", initialDoor, err),
		})
		_ = session.writeRender(ctx, s.defaultLobbyView(ctx, session.publicKey))
	}

	for {
		raw, err := s.readClientMessage(ctx, conn)
		if err != nil {
			status := websocket.CloseStatus(err)
			if status == websocket.StatusNormalClosure || errors.Is(err, context.Canceled) {
				return
			}
			return
		}
		if err := s.routeClientMessage(ctx, session, raw); err != nil {
			_ = session.write(ctx, protocol.ErrorMessageFor(err))
		}
	}
}

func (s *Server) initialDoorForReconnect(ctx context.Context, session *sessionState, pending *sessionState) (string, bool) {
	if pending == nil || pending.activeDoor == "" {
		return "lobby", false
	}
	if door, ok := s.findDoor(pending.activeDoor); ok && s.canAccessDoor(ctx, session, door) {
		return pending.activeDoor, true
	}
	s.completeSessionLeave(ctx, pending)
	return "lobby", false
}

func (s *Server) beginSessionDisconnect(session *sessionState) {
	if session == nil {
		return
	}
	sessionID := session.id
	session, immediate, ok := s.sessions.beginDisconnect(sessionID, s.disconnectGrace, func() {
		s.completePendingDisconnect(sessionID)
	})
	if !ok {
		return
	}
	if immediate {
		s.completeSessionLeave(context.Background(), session)
	}
}

func (s *Server) completePendingDisconnect(sessionID string) {
	session, ok := s.sessions.removePending(sessionID)
	if !ok {
		return
	}
	s.completeSessionLeave(context.Background(), session)
}

func (s *Server) completeSessionLeave(ctx context.Context, session *sessionState) {
	if session == nil || session.activeDoor == "" {
		return
	}
	door, ok := s.findDoor(session.activeDoor)
	if !ok {
		return
	}
	response, err := s.invokeDoorLifecycle(ctx, door, session, protocol.LifecycleOnLeave, nil)
	if err == nil {
		_ = s.applyDoorEffects(ctx, session, door, response, true)
	}
}

func (s *Server) sendDoorList(ctx context.Context, session *sessionState) error {
	return session.write(ctx, protocol.DoorListMessage{
		Type:  protocol.TypeDoorList,
		Doors: s.visibleDoorSummaries(ctx, session),
	})
}

func (s *Server) visibleDoorSummaries(ctx context.Context, session *sessionState) []protocol.DoorSummary {
	policy := s.loadStationPolicy(ctx)
	manifests := s.doorManifests()
	doors := make([]protocol.DoorSummary, 0, len(manifests))
	for _, door := range manifests {
		if !s.canAccessDoor(ctx, session, door) {
			continue
		}
		doors = append(doors, doorSummaryWithPolicy(doorSummary(door), policy))
	}
	sortDoorSummariesByPolicyOrder(doors, policy.DoorOrder)
	return doors
}

func (s *Server) allDoorSummaries(ctx context.Context) []protocol.DoorSummary {
	policy := s.loadStationPolicy(ctx)
	manifests := s.doorManifests()
	doors := make([]protocol.DoorSummary, 0, len(manifests))
	for _, door := range manifests {
		summary := doorSummaryWithPolicy(doorSummary(door), policy)
		summary = doorSummaryWithSettings(summary, door, policy.DoorSettings[door.ID])
		doors = append(doors, summary)
	}
	sortDoorSummariesByPolicyOrder(doors, policy.DoorOrder)
	return doors
}

func (s *Server) defaultLobbyView(ctx context.Context, publicKey string) protocol.UINode {
	session := &sessionState{publicKey: publicKey, role: s.roleForPublicKey(ctx, publicKey)}
	doors := s.visibleDoorSummaries(ctx, session)
	doorButtons := make([]protocol.UINode, 0, len(doors))
	for _, door := range doors {
		doorButtons = append(doorButtons, protocol.Button("open-"+door.ID, "Enter "+door.Name, "open_door:"+door.ID))
	}
	children := []protocol.UINode{
		protocol.Header(fmt.Sprintf("Welcome to %s", s.cfg.Name)),
		protocol.Panel("Connected",
			protocol.Text(fmt.Sprintf("Signed in as %s", identity.Fingerprint(publicKey))),
			protocol.Text("Pick a door to enter the station."),
		),
	}
	if len(doorButtons) == 0 {
		children = append(children, protocol.Panel("Doors",
			protocol.Text("No doors are currently available."),
		))
	} else {
		children = append(children, protocol.Panel("Doors", doorButtons...))
	}
	children = append(children,
		protocol.Status("Trusted client chrome stays local; doors render in the remote viewport."),
	)
	return protocol.Screen(children...)
}

func (s *Server) findDoor(id string) (runtime.DoorManifest, bool) {
	for _, door := range s.doorManifests() {
		if door.ID == id {
			return door, true
		}
	}
	return runtime.DoorManifest{}, false
}

func (s *Server) reloadDoorManifests(ctx context.Context) error {
	doorsDir, err := filepath.Abs(s.cfg.DoorsDir)
	if err != nil {
		return fmt.Errorf("resolve doors dir: %w", err)
	}
	manifests, err := runtime.LoadDoorManifests(doorsDir)
	if err != nil {
		return fmt.Errorf("reload door manifests: %w", err)
	}
	s.setDoorManifests(manifests)
	s.events.add("manifests_reloaded", adminDoorID, "", fmt.Sprintf("reloaded %d door manifests", len(manifests)))
	for _, session := range s.sessions.allSessions() {
		if err := s.sendDoorList(ctx, session); err != nil {
			return err
		}
	}
	return nil
}
