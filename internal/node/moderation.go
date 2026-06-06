package node

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"phosphornet/internal/protocol"
)

type userRateTracker struct {
	mu     sync.Mutex
	events map[string][]time.Time
	opens  map[string][]time.Time
}

func newUserRateTracker() *userRateTracker {
	return &userRateTracker{
		events: map[string][]time.Time{},
		opens:  map[string][]time.Time{},
	}
}

func (t *userRateTracker) allowEvent(publicKey string, limit int) bool {
	return t.allow(t.events, publicKey, limit)
}

func (t *userRateTracker) allowOpen(publicKey string, limit int) bool {
	return t.allow(t.opens, publicKey, limit)
}

func (t *userRateTracker) allow(buckets map[string][]time.Time, publicKey string, limit int) bool {
	if t == nil || limit <= 0 || publicKey == "" {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)
	values := buckets[publicKey]
	kept := values[:0]
	for _, value := range values {
		if value.After(cutoff) {
			kept = append(kept, value)
		}
	}
	if len(kept) >= limit {
		buckets[publicKey] = kept
		return false
	}
	kept = append(kept, now)
	buckets[publicKey] = kept
	return true
}

func (s *Server) moderationForPublicKey(ctx context.Context, publicKey string) (moderationEntry, bool, moderationEntry, bool, userRateLimit) {
	policy := s.loadStationPolicy(ctx)
	banEntry, banned := activeModerationEntry(policy.Moderation.BannedKeys[publicKey])
	muteEntry, muted := activeModerationEntry(policy.Moderation.MutedKeys[publicKey])
	return banEntry, banned, muteEntry, muted, policy.Moderation.RateLimits[publicKey]
}

func activeModerationEntry(entry moderationEntry) (moderationEntry, bool) {
	if entry.Reason == "" && entry.CreatedBy == "" && entry.CreatedAt == "" && entry.ExpiresAt == "" {
		return moderationEntry{}, false
	}
	if entry.ExpiresAt == "" {
		return entry, true
	}
	expires, err := time.Parse(time.RFC3339, entry.ExpiresAt)
	if err != nil {
		return entry, true
	}
	return entry, time.Now().UTC().Before(expires)
}

func newModerationEntry(session *sessionState, reason, expiresAt string) moderationEntry {
	createdBy := ""
	if session != nil {
		createdBy = session.publicKey
	}
	return moderationEntry{
		Reason:    strings.TrimSpace(reason),
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		ExpiresAt: strings.TrimSpace(expiresAt),
	}
}

func (s *Server) enforceOpenRateLimit(ctx context.Context, session *sessionState) error {
	if session == nil {
		return nil
	}
	_, _, _, _, limit := s.moderationForPublicKey(ctx, session.publicKey)
	if limit.OpensPerMinute <= 0 {
		return nil
	}
	if s.rateLimits.allowOpen(session.publicKey, limit.OpensPerMinute) {
		return nil
	}
	s.events.add("rate_limited", session.activeDoor, session.publicKey, fmt.Sprintf("open_door limit exceeded (%d/min)", limit.OpensPerMinute))
	return fmt.Errorf("open_door rate limit exceeded")
}

func (s *Server) enforceEventModeration(ctx context.Context, session *sessionState, event protocol.UIEvent) error {
	if session == nil {
		return nil
	}
	_, _, _, muted, limit := s.moderationForPublicKey(ctx, session.publicKey)
	if muted && !isPrivilegedRole(session.role) && !eventAllowedWhileMuted(event) {
		s.events.add("event_rejected", session.activeDoor, session.publicKey, "user is muted")
		return fmt.Errorf("user is muted")
	}
	if limit.EventsPerMinute <= 0 {
		return nil
	}
	if s.rateLimits.allowEvent(session.publicKey, limit.EventsPerMinute) {
		return nil
	}
	s.events.add("rate_limited", session.activeDoor, session.publicKey, fmt.Sprintf("event limit exceeded (%d/min)", limit.EventsPerMinute))
	return fmt.Errorf("event rate limit exceeded")
}

func eventAllowedWhileMuted(event protocol.UIEvent) bool {
	switch event.Kind {
	case protocol.EventKindFocus, protocol.EventKindKey:
		return true
	case protocol.EventKindAction, protocol.EventKindSelect:
		return strings.HasPrefix(event.Action, "nav:") || strings.HasPrefix(event.Action, "open_door:")
	default:
		return false
	}
}

func (r *sessionRegistry) closePublicKey(ctx context.Context, publicKey, reason string) {
	if r == nil || publicKey == "" {
		return
	}
	r.mu.Lock()
	targets := make([]*sessionState, 0)
	for _, session := range r.sessions {
		if session.publicKey == publicKey {
			session.forceImmediateLeave = true
			targets = append(targets, session)
		}
	}
	r.mu.Unlock()
	for _, session := range targets {
		if session.conn == nil {
			continue
		}
		_ = session.write(ctx, protocol.ErrorMessageFor(protocol.NewCodedError(protocol.ErrorAuth, reason, nil)))
		_ = session.conn.Close(websocket.StatusPolicyViolation, reason)
	}
}
