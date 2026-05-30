package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
	"phosphornet/internal/storage"
)

type adminOpResult struct {
	policyChanged       bool
	doorSettingsChanged bool
	reloadedManifests   bool
	clearedEvents       bool
	bannedKeys          []string
}

func validateAdminOps(session *sessionState, door runtime.DoorManifest, ops []protocol.AdminOp) error {
	for _, op := range ops {
		capability, err := adminOpCapability(op)
		if err != nil {
			return err
		}
		if err := requirePrivilegedAdminCapability(session, door, capability); err != nil {
			return err
		}
	}
	return nil
}

func adminOpCapability(op protocol.AdminOp) (string, error) {
	switch op.Op {
	case "set_user_role":
		return runtime.CapabilityAdminSetUserRoles, nil
	case "set_door_roles", "set_door_enabled":
		return runtime.CapabilityAdminSetDoorPolicy, nil
	case "set_door_setting":
		return runtime.CapabilityAdminSetDoorSettings, nil
	case "reload_manifests":
		return runtime.CapabilityAdminReloadManifests, nil
	case "reorder_doors":
		return runtime.CapabilityAdminReorderDoors, nil
	case "set_station_notice", "clear_station_notices":
		return runtime.CapabilityAdminSetStationNotice, nil
	case "set_maintenance", "record_maintenance_checkpoint", "reset_maintenance":
		return runtime.CapabilityAdminSetMaintenance, nil
	case "clear_event_log":
		return runtime.CapabilityAdminReadLogs, nil
	case "ban_key", "unban_key", "mute_key", "unmute_key", "set_user_rate_limit", "record_moderation_note":
		return runtime.CapabilityAdminModerateUsers, nil
	default:
		return "", fmt.Errorf("unknown admin op %q", op.Op)
	}
}

func (s *Server) applyAdminOps(ctx context.Context, session *sessionState, door runtime.DoorManifest, ops []protocol.AdminOp) (adminOpResult, error) {
	result := adminOpResult{}
	if len(ops) == 0 {
		return result, nil
	}
	if err := validateAdminOps(session, door, ops); err != nil {
		return result, err
	}
	policy := s.loadStationPolicy(ctx)
	if policy.Moderation.BannedKeys == nil {
		policy.Moderation.BannedKeys = map[string]moderationEntry{}
	}
	if policy.Moderation.MutedKeys == nil {
		policy.Moderation.MutedKeys = map[string]moderationEntry{}
	}
	if policy.Moderation.RateLimits == nil {
		policy.Moderation.RateLimits = map[string]userRateLimit{}
	}
	pendingAudit := []storage.AuditEvent{}
	for _, op := range ops {
		switch op.Op {
		case "set_user_role":
			publicKey := strings.TrimSpace(op.PublicKey)
			if publicKey == "" {
				return result, fmt.Errorf("set_user_role requires public_key")
			}
			if policy.Roles == nil {
				policy.Roles = map[string]string{}
			}
			role := normalizeRole(op.Role)
			if role == "" {
				delete(policy.Roles, publicKey)
				s.events.add("admin_action", door.ID, session.publicKey, "removed role for "+publicKey)
			} else {
				policy.Roles[publicKey] = role
				s.events.add("admin_action", door.ID, session.publicKey, "set role "+role+" for "+publicKey)
			}
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.set_user_role", publicKey, "success", map[string]any{
				"role":        role,
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "set_door_roles":
			doorID := strings.TrimSpace(op.DoorID)
			if doorID == "" {
				return result, fmt.Errorf("set_door_roles requires door_id")
			}
			if policy.DoorRoles == nil {
				policy.DoorRoles = map[string][]string{}
			}
			roles := sortedUniqueRoles(op.Roles)
			if len(roles) == 0 {
				delete(policy.DoorRoles, doorID)
				s.events.add("admin_action", door.ID, session.publicKey, "cleared role policy for door "+doorID)
			} else {
				policy.DoorRoles[doorID] = roles
				s.events.add("admin_action", door.ID, session.publicKey, "set role policy for door "+doorID)
			}
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.set_door_roles", doorID, "success", map[string]any{
				"roles":       roles,
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "set_door_enabled":
			doorID := strings.TrimSpace(op.DoorID)
			if doorID == "" {
				return result, fmt.Errorf("set_door_enabled requires door_id")
			}
			if doorID == adminDoorID {
				continue
			}
			if op.Enabled == nil {
				return result, fmt.Errorf("set_door_enabled requires enabled")
			}
			if policy.DisabledDoors == nil {
				policy.DisabledDoors = map[string]bool{}
			}
			if *op.Enabled {
				delete(policy.DisabledDoors, doorID)
				s.events.add("admin_action", door.ID, session.publicKey, "enabled door "+doorID)
			} else {
				policy.DisabledDoors[doorID] = true
				s.events.add("admin_action", door.ID, session.publicKey, "disabled door "+doorID)
			}
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.set_door_enabled", doorID, "success", map[string]any{
				"enabled":     *op.Enabled,
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "reorder_doors":
			policy.DoorOrder = orderedStringSliceFromAny(op.DoorOrder)
			s.events.add("admin_action", door.ID, session.publicKey, "reordered doors")
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.reorder_doors", "doors", "success", map[string]any{
				"door_order":  policy.DoorOrder,
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "set_door_setting":
			doorID := strings.TrimSpace(op.DoorID)
			key := strings.TrimSpace(op.SettingKey)
			if doorID == "" || key == "" {
				return result, fmt.Errorf("set_door_setting requires door_id and setting_key")
			}
			targetDoor, ok := s.findDoor(doorID)
			if !ok {
				return result, fmt.Errorf("unknown door %q", doorID)
			}
			setting, ok := targetDoor.Settings[key]
			if !ok {
				return result, fmt.Errorf("unknown setting %q for door %q", key, doorID)
			}
			if policy.DoorSettings == nil {
				policy.DoorSettings = map[string]map[string]any{}
			}
			doorSettings := copyAnyMap(policy.DoorSettings[doorID])
			if op.Reset {
				delete(doorSettings, key)
			} else {
				value, ok := runtime.CoerceDoorSettingValue(setting, op.SettingValue)
				if !ok {
					return result, fmt.Errorf("setting %q does not accept value", key)
				}
				doorSettings[key] = value
			}
			if len(doorSettings) == 0 {
				delete(policy.DoorSettings, doorID)
			} else {
				policy.DoorSettings[doorID] = doorSettings
			}
			s.events.add("admin_action", door.ID, session.publicKey, "updated setting "+doorID+"."+key)
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.set_door_setting", doorID+"."+key, "success", map[string]any{
				"door_id":     doorID,
				"setting_key": key,
				"reset":       op.Reset,
				"value":       op.SettingValue,
				"source_door": door.ID,
			}))
			result.doorSettingsChanged = true
		case "reload_manifests":
			if err := s.reloadDoorManifests(ctx); err != nil {
				return result, err
			}
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.reload_manifests", s.cfg.DoorsDir, "success", map[string]any{
				"door_count":  len(s.doorManifests()),
				"source_door": door.ID,
			}))
			result.reloadedManifests = true
		case "set_station_notice":
			message := strings.TrimSpace(op.Message)
			if message == "" {
				return result, fmt.Errorf("set_station_notice requires message")
			}
			policy.Notices = append(policy.Notices, message)
			s.events.add("admin_action", door.ID, session.publicKey, "posted station notice")
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.set_station_notice", "station", "success", map[string]any{
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "clear_station_notices":
			policy.Notices = []string{}
			s.events.add("admin_action", door.ID, session.publicKey, "cleared station notices")
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.clear_station_notices", "station", "success", map[string]any{
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "set_maintenance":
			if op.Maintenance == nil {
				return result, fmt.Errorf("set_maintenance requires maintenance")
			}
			policy.MaintenanceMode = *op.Maintenance
			s.events.add("admin_action", door.ID, session.publicKey, fmt.Sprintf("set maintenance mode to %t", *op.Maintenance))
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.set_maintenance", "station", "success", map[string]any{
				"maintenance": *op.Maintenance,
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "record_maintenance_checkpoint":
			policy.MaintenanceCount++
			s.events.add("admin_action", door.ID, session.publicKey, "recorded maintenance checkpoint")
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.record_maintenance_checkpoint", "station", "success", map[string]any{
				"maintenance_count": policy.MaintenanceCount,
				"source_door":       door.ID,
			}))
			result.policyChanged = true
		case "reset_maintenance":
			policy.MaintenanceMode = false
			policy.MaintenanceCount = 0
			policy.Notices = []string{}
			s.events.add("admin_action", door.ID, session.publicKey, "reset maintenance state")
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.reset_maintenance", "station", "success", map[string]any{
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "clear_event_log":
			s.events.clear()
			s.events.add("admin_action", door.ID, session.publicKey, "cleared in-memory event log")
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.clear_event_log", "runtime_event_log", "success", map[string]any{
				"source_door": door.ID,
			}))
			result.clearedEvents = true
		case "ban_key":
			publicKey := strings.TrimSpace(op.PublicKey)
			if publicKey == "" {
				return result, fmt.Errorf("ban_key requires public_key")
			}
			policy.Moderation.BannedKeys[publicKey] = newModerationEntry(session, firstNonEmptyString(op.Reason, op.Message), op.ExpiresAt)
			s.events.add("moderation_action", door.ID, session.publicKey, "banned key "+publicKey)
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.ban_key", publicKey, "success", map[string]any{
				"reason":      firstNonEmptyString(op.Reason, op.Message),
				"expires_at":  op.ExpiresAt,
				"source_door": door.ID,
			}))
			result.bannedKeys = append(result.bannedKeys, publicKey)
			result.policyChanged = true
		case "unban_key":
			publicKey := strings.TrimSpace(op.PublicKey)
			if publicKey == "" {
				return result, fmt.Errorf("unban_key requires public_key")
			}
			delete(policy.Moderation.BannedKeys, publicKey)
			s.events.add("moderation_action", door.ID, session.publicKey, "unbanned key "+publicKey)
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.unban_key", publicKey, "success", map[string]any{
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "mute_key":
			publicKey := strings.TrimSpace(op.PublicKey)
			if publicKey == "" {
				return result, fmt.Errorf("mute_key requires public_key")
			}
			policy.Moderation.MutedKeys[publicKey] = newModerationEntry(session, firstNonEmptyString(op.Reason, op.Message), op.ExpiresAt)
			s.events.add("moderation_action", door.ID, session.publicKey, "muted key "+publicKey)
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.mute_key", publicKey, "success", map[string]any{
				"reason":      firstNonEmptyString(op.Reason, op.Message),
				"expires_at":  op.ExpiresAt,
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "unmute_key":
			publicKey := strings.TrimSpace(op.PublicKey)
			if publicKey == "" {
				return result, fmt.Errorf("unmute_key requires public_key")
			}
			delete(policy.Moderation.MutedKeys, publicKey)
			s.events.add("moderation_action", door.ID, session.publicKey, "unmuted key "+publicKey)
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.unmute_key", publicKey, "success", map[string]any{
				"source_door": door.ID,
			}))
			result.policyChanged = true
		case "set_user_rate_limit":
			publicKey := strings.TrimSpace(op.PublicKey)
			if publicKey == "" {
				return result, fmt.Errorf("set_user_rate_limit requires public_key")
			}
			if op.Reset {
				delete(policy.Moderation.RateLimits, publicKey)
				s.events.add("moderation_action", door.ID, session.publicKey, "cleared rate limit for "+publicKey)
			} else {
				limit := userRateLimit{}
				if op.EventsPerMinute != nil {
					limit.EventsPerMinute = maxInt(*op.EventsPerMinute, 0)
				}
				if op.OpensPerMinute != nil {
					limit.OpensPerMinute = maxInt(*op.OpensPerMinute, 0)
				}
				if limit.EventsPerMinute == 0 && limit.OpensPerMinute == 0 {
					delete(policy.Moderation.RateLimits, publicKey)
				} else {
					policy.Moderation.RateLimits[publicKey] = limit
				}
				s.events.add("moderation_action", door.ID, session.publicKey, "updated rate limit for "+publicKey)
			}
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.set_user_rate_limit", publicKey, "success", map[string]any{
				"reset":             op.Reset,
				"events_per_minute": op.EventsPerMinute,
				"opens_per_minute":  op.OpensPerMinute,
				"source_door":       door.ID,
			}))
			result.policyChanged = true
		case "record_moderation_note":
			publicKey := strings.TrimSpace(op.PublicKey)
			message := strings.TrimSpace(firstNonEmptyString(op.Message, op.Reason))
			if publicKey == "" || message == "" {
				return result, fmt.Errorf("record_moderation_note requires public_key and message")
			}
			policy.Moderation.ModerationNotes = append(policy.Moderation.ModerationNotes, moderationNote{
				PublicKey: publicKey,
				Message:   message,
				CreatedBy: session.publicKey,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			})
			if len(policy.Moderation.ModerationNotes) > 100 {
				policy.Moderation.ModerationNotes = append([]moderationNote{}, policy.Moderation.ModerationNotes[len(policy.Moderation.ModerationNotes)-100:]...)
			}
			s.events.add("moderation_action", door.ID, session.publicKey, "recorded moderation note for "+publicKey)
			pendingAudit = append(pendingAudit, auditEvent(session.publicKey, "admin.record_moderation_note", publicKey, "success", map[string]any{
				"source_door": door.ID,
			}))
			result.policyChanged = true
		}
	}
	if result.policyChanged || result.doorSettingsChanged {
		if err := s.saveStationPolicy(ctx, policy); err != nil {
			return result, err
		}
	}
	for _, event := range pendingAudit {
		s.audit(ctx, event)
	}
	for _, publicKey := range result.bannedKeys {
		s.sessions.closePublicKey(ctx, publicKey, "station access revoked")
	}
	return result, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func maxInt(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}

func copyAnyMap(values map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range values {
		result[key] = value
	}
	return result
}
