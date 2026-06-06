package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"phosphornet/internal/protocol"
)

type sessionState struct {
	id                  string
	publicKey           string
	role                string
	activeDoor          string
	roomID              string
	conn                *websocket.Conn
	writeMu             sync.Mutex
	viewMu              sync.RWMutex
	viewPolicy          renderEventPolicy
	renderRev           int64
	eventIDs            map[string]time.Time
	disconnected        bool
	disconnectedAt      time.Time
	disconnectTimer     *time.Timer
	forceImmediateLeave bool
}

const sessionEventIDWindow = 2 * time.Minute

func (s *sessionState) write(ctx context.Context, message any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return wsjson.Write(ctx, s.conn, message)
}

func (s *sessionState) writeRender(ctx context.Context, view protocol.UINode) error {
	policy, err := buildRenderEventPolicy(view)
	if err != nil {
		return fmt.Errorf("validate render tree: %w", err)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.viewMu.Lock()
	s.renderRev++
	revision := s.renderRev
	s.viewPolicy = policy
	s.viewMu.Unlock()
	if err := wsjson.Write(ctx, s.conn, protocol.RenderMessage{
		Type:           protocol.TypeRender,
		SessionID:      s.id,
		ActiveDoorID:   s.activeDoor,
		RenderRevision: revision,
		View:           view,
	}); err != nil {
		return err
	}

	return nil
}

func (s *sessionState) validateEvent(event protocol.UIEvent) error {
	s.viewMu.RLock()
	policy := s.viewPolicy
	s.viewMu.RUnlock()
	return validateEventAgainstPolicy(event, policy)
}

func (s *sessionState) validateEventEnvelope(message protocol.EventMessage, now time.Time) error {
	if message.SessionID != s.id {
		return fmt.Errorf("stale or mismatched session id")
	}
	if message.ActiveDoorID != s.activeDoor {
		return fmt.Errorf("stale or mismatched active door id")
	}
	if message.EventID == "" {
		return fmt.Errorf("event id is required")
	}
	if message.RenderRevision <= 0 {
		return fmt.Errorf("render revision is required")
	}
	if s.claimEventID(message.EventID, now) {
		return fmt.Errorf("duplicate event id")
	}
	if submitLikeEvent(message.Event.Kind) && message.RenderRevision != s.currentRenderRevision() {
		return fmt.Errorf("stale render revision")
	}
	return nil
}

func (s *sessionState) currentRenderRevision() int64 {
	s.viewMu.RLock()
	defer s.viewMu.RUnlock()
	return s.renderRev
}

func (s *sessionState) claimEventID(eventID string, now time.Time) bool {
	s.viewMu.Lock()
	defer s.viewMu.Unlock()
	if s.eventIDs == nil {
		s.eventIDs = map[string]time.Time{}
	}
	cutoff := now.Add(-sessionEventIDWindow)
	for seenID, seenAt := range s.eventIDs {
		if seenAt.Before(cutoff) {
			delete(s.eventIDs, seenID)
		}
	}
	if _, ok := s.eventIDs[eventID]; ok {
		return true
	}
	s.eventIDs[eventID] = now
	return false
}

func submitLikeEvent(kind protocol.EventKind) bool {
	switch kind {
	case protocol.EventKindAction, protocol.EventKindSelect, protocol.EventKindSubmit:
		return true
	default:
		return false
	}
}

func (s *sessionState) presenceUser() protocol.PresenceUser {
	return protocol.PresenceUser{
		PublicKey:   s.publicKey,
		Fingerprint: fingerprintForPublicKey(s.publicKey),
		Role:        s.role,
		ActiveDoor:  s.activeDoor,
	}
}

type sessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*sessionState
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{sessions: map[string]*sessionState{}}
}

func (r *sessionRegistry) add(session *sessionState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session.disconnected = false
	session.disconnectedAt = time.Time{}
	session.disconnectTimer = nil
	r.sessions[session.id] = session
}

func (r *sessionRegistry) remove(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session, ok := r.sessions[sessionID]; ok && session.disconnectTimer != nil {
		session.disconnectTimer.Stop()
	}
	delete(r.sessions, sessionID)
}

func (r *sessionRegistry) beginDisconnect(sessionID string, grace time.Duration, onGraceExpired func()) (*sessionState, bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok || session.disconnected {
		return nil, false, false
	}
	session.disconnected = true
	session.disconnectedAt = time.Now()
	session.conn = nil
	if session.forceImmediateLeave || grace <= 0 {
		delete(r.sessions, sessionID)
		return session, true, true
	}
	session.disconnectTimer = time.AfterFunc(grace, onGraceExpired)
	return session, false, true
}

func (r *sessionRegistry) removePending(sessionID string) (*sessionState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok || !session.disconnected {
		return nil, false
	}
	if session.disconnectTimer != nil {
		session.disconnectTimer.Stop()
		session.disconnectTimer = nil
	}
	delete(r.sessions, sessionID)
	return session, true
}

func (r *sessionRegistry) claimPendingReconnect(publicKey string) *sessionState {
	if publicKey == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var claimed *sessionState
	for _, session := range r.sessions {
		if !session.disconnected || session.publicKey != publicKey {
			continue
		}
		if claimed == nil || session.disconnectedAt.After(claimed.disconnectedAt) {
			claimed = session
		}
	}
	if claimed == nil {
		return nil
	}
	if claimed.disconnectTimer != nil {
		claimed.disconnectTimer.Stop()
		claimed.disconnectTimer = nil
	}
	delete(r.sessions, claimed.id)
	return claimed
}

func (r *sessionRegistry) updateDoor(sessionID, doorID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[sessionID]
	if !ok {
		return
	}
	session.activeDoor = doorID
	session.roomID = implicitRoomID(doorID)
}

func (r *sessionRegistry) refreshRoles(roleForPublicKey func(string) string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, session := range r.sessions {
		if session.disconnected {
			continue
		}
		session.role = roleForPublicKey(session.publicKey)
	}
}

func (r *sessionRegistry) presence(roomID, doorID string) protocol.PresenceSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roomUsers := []protocol.PresenceUser{}
	doorUsers := []protocol.PresenceUser{}
	for _, session := range r.sessions {
		if session.disconnected {
			continue
		}
		if session.roomID == roomID {
			roomUsers = append(roomUsers, session.presenceUser())
		}
		if session.activeDoor == doorID {
			doorUsers = append(doorUsers, session.presenceUser())
		}
	}
	return protocol.PresenceSnapshot{
		RoomUsers: roomUsers,
		DoorUsers: doorUsers,
	}
}

func (r *sessionRegistry) allPresence() []protocol.PresenceUser {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := []protocol.PresenceUser{}
	for _, session := range r.sessions {
		if session.disconnected {
			continue
		}
		users = append(users, session.presenceUser())
	}
	return users
}

func (r *sessionRegistry) allSessions() []*sessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]*sessionState, 0, len(r.sessions))
	for _, session := range r.sessions {
		if session.disconnected {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions
}

func (r *sessionRegistry) targets(source *sessionState, doorID string, effect protocol.BroadcastEffect) []*sessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roomID := effect.RoomID
	if roomID == "" {
		roomID = implicitRoomID(doorID)
	}
	targetDoorID := effect.DoorID
	if targetDoorID == "" {
		targetDoorID = doorID
	}

	var sessions []*sessionState
	for _, session := range r.sessions {
		if session.disconnected {
			continue
		}
		switch effect.Scope {
		case protocol.BroadcastScopeUser:
			if session.publicKey == effect.UserPublicKey {
				sessions = append(sessions, session)
			}
		case protocol.BroadcastScopeDoor:
			if session.activeDoor == targetDoorID {
				sessions = append(sessions, session)
			}
		case protocol.BroadcastScopeRoom, "":
			if session.roomID == roomID {
				sessions = append(sessions, session)
			}
		default:
			if source != nil && session.id == source.id {
				sessions = append(sessions, session)
			}
		}
	}
	return sessions
}

func (r *sessionRegistry) notifyTargets(source *sessionState, doorID string, effect protocol.NotifyEffect) []*sessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*sessionState
	for _, session := range r.sessions {
		if session.disconnected {
			continue
		}
		switch effect.Target {
		case protocol.NotifyTargetUser:
			if session.publicKey == effect.UserPublicKey {
				sessions = append(sessions, session)
			}
		case protocol.NotifyTargetAll:
			sessions = append(sessions, session)
		case protocol.NotifyTargetDoor:
			if session.activeDoor == doorID {
				sessions = append(sessions, session)
			}
		case protocol.NotifyTargetRoom:
			if session.roomID == implicitRoomID(doorID) {
				sessions = append(sessions, session)
			}
		case protocol.NotifyTargetSelf, "":
			if source != nil && session.id == source.id {
				sessions = append(sessions, session)
			}
		}
	}
	return sessions
}
