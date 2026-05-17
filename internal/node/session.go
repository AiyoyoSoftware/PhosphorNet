package node

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"phosphornet/internal/protocol"
)

type sessionState struct {
	id         string
	publicKey  string
	role       string
	activeDoor string
	roomID     string
	conn       *websocket.Conn
	writeMu    sync.Mutex
	viewMu     sync.RWMutex
	viewPolicy renderEventPolicy
}

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
	if err := wsjson.Write(ctx, s.conn, protocol.RenderMessage{
		Type:      protocol.TypeRender,
		SessionID: s.id,
		View:      view,
	}); err != nil {
		return err
	}

	s.viewMu.Lock()
	s.viewPolicy = policy
	s.viewMu.Unlock()
	return nil
}

func (s *sessionState) validateEvent(event protocol.UIEvent) error {
	s.viewMu.RLock()
	policy := s.viewPolicy
	s.viewMu.RUnlock()
	return validateEventAgainstPolicy(event, policy)
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
	r.sessions[session.id] = session
}

func (r *sessionRegistry) remove(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionID)
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
		session.role = roleForPublicKey(session.publicKey)
	}
}

func (r *sessionRegistry) presence(roomID, doorID string) protocol.PresenceSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roomUsers := []protocol.PresenceUser{}
	doorUsers := []protocol.PresenceUser{}
	for _, session := range r.sessions {
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
		users = append(users, session.presenceUser())
	}
	return users
}

func (r *sessionRegistry) allSessions() []*sessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]*sessionState, 0, len(r.sessions))
	for _, session := range r.sessions {
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
