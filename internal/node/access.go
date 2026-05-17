package node

import (
	"context"
	"sort"
	"strings"

	"phosphornet/internal/identity"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
)

const (
	accessPublic     = "public"
	accessInviteOnly = "invite_only"
	accessAdmin      = "admin"
	accessRole       = "role"

	visibilityPublic  = "public"
	visibilityPrivate = "private"
	visibilityHidden  = "hidden"
)

func normalizeAccess(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", accessPublic:
		return accessPublic
	case accessInviteOnly:
		return accessInviteOnly
	case accessAdmin:
		return accessAdmin
	case accessRole:
		return accessRole
	default:
		return accessInviteOnly
	}
}

func normalizeVisibility(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", visibilityPublic:
		return visibilityPublic
	case visibilityPrivate:
		return visibilityPrivate
	case visibilityHidden:
		return visibilityHidden
	default:
		return visibilityPrivate
	}
}

func allowlistContains(allowlist []string, publicKey string) bool {
	fingerprint := identity.Fingerprint(publicKey)
	for _, entry := range allowlist {
		normalized := strings.TrimSpace(entry)
		if normalized == "" {
			continue
		}
		if normalized == publicKey || strings.EqualFold(normalized, fingerprint) {
			return true
		}
	}
	return false
}

func (s *Server) stationAllows(ctx context.Context, publicKey string) bool {
	_, banned, _, _, _ := s.moderationForPublicKey(ctx, publicKey)
	if banned {
		return false
	}
	if s.isAdmin(publicKey) {
		return true
	}
	switch normalizeAccess(s.cfg.Access.Mode) {
	case accessPublic:
		return true
	case accessInviteOnly:
		return allowlistContains(s.cfg.Access.Allowlist, publicKey)
	default:
		return false
	}
}

func (s *Server) roleForPublicKey(ctx context.Context, publicKey string) string {
	return s.roleForPublicKeyWithPolicy(publicKey, s.loadStationPolicy(ctx))
}

func (s *Server) roleForPublicKeyWithPolicy(publicKey string, policy stationPolicy) string {
	if s.isAdmin(publicKey) {
		return "admin"
	}
	if role := normalizeRole(policy.Roles[publicKey]); role != "" {
		return role
	}
	return "member"
}

func (s *Server) isAdmin(publicKey string) bool {
	return allowlistContains(s.cfg.Access.Admins, publicKey)
}

func (s *Server) canAccessDoor(ctx context.Context, session *sessionState, door runtime.DoorManifest) bool {
	if session == nil {
		return false
	}
	policy := s.loadStationPolicy(ctx)
	if _, banned := activeModerationEntry(policy.Moderation.BannedKeys[session.publicKey]); banned {
		return false
	}
	if policy.DisabledDoors[door.ID] && door.ID != adminDoorID {
		return false
	}
	if isPrivilegedRole(session.role) {
		return true
	}
	if roles, ok := doorRolesForPolicy(policy, door.ID); ok {
		return roleInList(session.role, roles)
	}
	switch normalizeAccess(door.Access) {
	case accessPublic:
		return true
	case accessInviteOnly:
		return allowlistContains(door.Allowlist, session.publicKey)
	case accessAdmin:
		return isPrivilegedRole(session.role)
	default:
		return false
	}
}

func doorSummary(door runtime.DoorManifest) protocol.DoorSummary {
	return protocol.DoorSummary{
		ID:             door.ID,
		Name:           door.Name,
		Runtime:        runtimeNameForManifest(door),
		Visibility:     normalizeVisibility(door.Visibility),
		Access:         normalizeAccess(door.Access),
		AllowlistCount: len(door.Allowlist),
		Entry:          door.Entry,
		Command:        append([]string{}, door.Command...),
		SandboxProfile: runtime.NormalizeSandboxProfileForDisplay(door.Sandbox.Profile),
	}
}

func doorSummaryWithSettings(summary protocol.DoorSummary, door runtime.DoorManifest, overrides map[string]any) protocol.DoorSummary {
	if len(door.Settings) == 0 {
		return summary
	}
	resolved := runtime.ResolveDoorSettings(door, overrides)
	names := make([]string, 0, len(door.Settings))
	for name := range door.Settings {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		setting := door.Settings[name]
		defaultValue, _ := runtime.CoerceDoorSettingValue(setting, setting.Default)
		summary.Settings = append(summary.Settings, protocol.DoorSettingSummary{
			Name:    name,
			Type:    setting.Type,
			Label:   setting.Label,
			Default: defaultValue,
			Value:   resolved[name],
			Options: append([]string{}, setting.Options...),
		})
	}
	return summary
}

func runtimeNameForManifest(door runtime.DoorManifest) string {
	if door.Runtime != "" {
		return strings.ToLower(door.Runtime)
	}
	switch {
	case strings.HasSuffix(strings.ToLower(door.Entry), ".py"):
		return "python"
	default:
		return "lua"
	}
}
