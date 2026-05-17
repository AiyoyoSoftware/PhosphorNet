package node

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"phosphornet/internal/protocol"
	"phosphornet/internal/storage"
)

const adminDoorID = "admin"
const stationPolicyNodeStateKey = "station.policy"

type stationPolicy struct {
	Roles            map[string]string
	DoorRoles        map[string][]string
	DisabledDoors    map[string]bool
	DoorOrder        []string
	DoorSettings     map[string]map[string]any
	Moderation       moderationPolicy
	MaintenanceMode  bool
	MaintenanceCount int
	Notices          []string
}

type moderationPolicy struct {
	BannedKeys      map[string]moderationEntry
	MutedKeys       map[string]moderationEntry
	RateLimits      map[string]userRateLimit
	ModerationNotes []moderationNote
}

type moderationEntry struct {
	Reason    string `json:"reason,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type userRateLimit struct {
	EventsPerMinute int `json:"events_per_minute,omitempty"`
	OpensPerMinute  int `json:"opens_per_minute,omitempty"`
}

type moderationNote struct {
	PublicKey string `json:"public_key,omitempty"`
	Message   string `json:"message,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func (s *Server) loadStationPolicy(ctx context.Context) stationPolicy {
	policy := stationPolicy{
		Roles:         map[string]string{},
		DoorRoles:     map[string][]string{},
		DisabledDoors: map[string]bool{},
		DoorOrder:     []string{},
		DoorSettings:  map[string]map[string]any{},
		Moderation: moderationPolicy{
			BannedKeys: map[string]moderationEntry{},
			MutedKeys:  map[string]moderationEntry{},
			RateLimits: map[string]userRateLimit{},
		},
		Notices: []string{},
	}
	if s == nil || s.store == nil {
		return policy
	}
	value, err := s.store.LoadNodeState(ctx, stationPolicyNodeStateKey)
	if err != nil {
		return policy
	}
	if len(value) == 0 {
		legacyPolicy, legacyOK := s.loadLegacyStationPolicy(ctx, policy)
		if legacyOK {
			_ = s.saveStationPolicy(ctx, legacyPolicy)
			return legacyPolicy
		}
		return policy
	}
	policy.Roles = stringMapFromAnyMap(value["roles"])
	policy.DoorRoles = stringSliceMapFromAnyMap(value["door_roles"])
	policy.DisabledDoors = boolMapFromAnyMap(value["disabled_doors"])
	policy.DoorOrder = orderedStringSliceFromAny(value["door_order"])
	policy.DoorSettings = anyMapMapFromAny(value["door_settings"])
	policy.Moderation = moderationPolicyFromAny(value["moderation"])
	if enabled, ok := value["maintenance_mode"].(bool); ok {
		policy.MaintenanceMode = enabled
	}
	policy.MaintenanceCount = intFromAny(value["maintenance_count"])
	policy.Notices = orderedStringSliceFromAny(value["notices"])
	return policy
}

func (s *Server) loadLegacyStationPolicy(ctx context.Context, fallback stationPolicy) (stationPolicy, bool) {
	state, err := s.store.LoadScopedState(ctx, adminDoorID, storage.StateScopeIDs{Global: "global"})
	if err != nil || len(state.Global) == 0 {
		return fallback, false
	}
	fallback.Roles = stringMapFromAnyMap(state.Global["roles"])
	fallback.DoorRoles = stringSliceMapFromAnyMap(state.Global["door_roles"])
	fallback.DisabledDoors = boolMapFromAnyMap(state.Global["disabled_doors"])
	fallback.DoorOrder = orderedStringSliceFromAny(state.Global["door_order"])
	fallback.DoorSettings = anyMapMapFromAny(state.Global["door_settings"])
	fallback.Moderation = moderationPolicyFromAny(state.Global["moderation"])
	if enabled, ok := state.Global["maintenance_mode"].(bool); ok {
		fallback.MaintenanceMode = enabled
	}
	fallback.MaintenanceCount = intFromAny(state.Global["maintenance_count"])
	fallback.Notices = orderedStringSliceFromAny(state.Global["notices"])
	return fallback, true
}

func (s *Server) saveStationPolicy(ctx context.Context, policy stationPolicy) error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.SaveNodeState(ctx, stationPolicyNodeStateKey, stationPolicyToMap(policy))
}

func stationPolicyToMap(policy stationPolicy) map[string]any {
	return map[string]any{
		"roles":             policy.Roles,
		"door_roles":        policy.DoorRoles,
		"disabled_doors":    policy.DisabledDoors,
		"door_order":        policy.DoorOrder,
		"door_settings":     policy.DoorSettings,
		"moderation":        moderationPolicyToMap(policy.Moderation),
		"maintenance_mode":  policy.MaintenanceMode,
		"maintenance_count": policy.MaintenanceCount,
		"notices":           policy.Notices,
	}
}

func moderationPolicyFromAny(value any) moderationPolicy {
	raw, _ := value.(map[string]any)
	return moderationPolicy{
		BannedKeys:      moderationEntriesFromAny(raw["banned_keys"]),
		MutedKeys:       moderationEntriesFromAny(raw["muted_keys"]),
		RateLimits:      userRateLimitsFromAny(raw["rate_limits"]),
		ModerationNotes: moderationNotesFromAny(raw["notes"]),
	}
}

func moderationPolicyToMap(policy moderationPolicy) map[string]any {
	return map[string]any{
		"banned_keys": moderationEntriesToMap(policy.BannedKeys),
		"muted_keys":  moderationEntriesToMap(policy.MutedKeys),
		"rate_limits": userRateLimitsToMap(policy.RateLimits),
		"notes":       moderationNotesToSlice(policy.ModerationNotes),
	}
}

func normalizeRole(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isPrivilegedRole(role string) bool {
	switch normalizeRole(role) {
	case "admin", "sysop":
		return true
	default:
		return false
	}
}

func roleInList(role string, roles []string) bool {
	role = normalizeRole(role)
	for _, candidate := range roles {
		if normalizeRole(candidate) == role {
			return true
		}
	}
	return false
}

func sortedUniqueRoles(values []string) []string {
	seen := map[string]bool{}
	roles := make([]string, 0, len(values))
	for _, value := range values {
		role := normalizeRole(value)
		if role == "" || seen[role] {
			continue
		}
		seen[role] = true
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func stringMapFromAnyMap(value any) map[string]string {
	result := map[string]string{}
	raw, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, value := range raw {
		str, ok := value.(string)
		if !ok {
			str = fmt.Sprint(value)
		}
		key = strings.TrimSpace(key)
		str = normalizeRole(str)
		if key != "" && str != "" {
			result[key] = str
		}
	}
	return result
}

func stringSliceMapFromAnyMap(value any) map[string][]string {
	result := map[string][]string{}
	raw, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		roles := stringSliceFromAny(value)
		if len(roles) > 0 {
			result[key] = roles
		}
	}
	return result
}

func boolMapFromAnyMap(value any) map[string]bool {
	result := map[string]bool{}
	raw, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		switch v := value.(type) {
		case bool:
			result[key] = v
		case string:
			result[key] = strings.EqualFold(strings.TrimSpace(v), "true")
		}
	}
	return result
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var result int
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &result); err == nil {
			return result
		}
	}
	return 0
}

func anyMapMapFromAny(value any) map[string]map[string]any {
	result := map[string]map[string]any{}
	raw, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		nested, ok := value.(map[string]any)
		if !ok {
			continue
		}
		result[key] = nested
	}
	return result
}

func moderationEntriesFromAny(value any) map[string]moderationEntry {
	result := map[string]moderationEntry{}
	raw, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		entryMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		result[key] = moderationEntry{
			Reason:    stringFromAny(entryMap["reason"]),
			CreatedBy: stringFromAny(entryMap["created_by"]),
			CreatedAt: stringFromAny(entryMap["created_at"]),
			ExpiresAt: stringFromAny(entryMap["expires_at"]),
		}
	}
	return result
}

func moderationEntriesToMap(entries map[string]moderationEntry) map[string]any {
	result := map[string]any{}
	for key, entry := range entries {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = map[string]any{
			"reason":     entry.Reason,
			"created_by": entry.CreatedBy,
			"created_at": entry.CreatedAt,
			"expires_at": entry.ExpiresAt,
		}
	}
	return result
}

func userRateLimitsFromAny(value any) map[string]userRateLimit {
	result := map[string]userRateLimit{}
	raw, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		limitMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		limit := userRateLimit{
			EventsPerMinute: intFromAny(limitMap["events_per_minute"]),
			OpensPerMinute:  intFromAny(limitMap["opens_per_minute"]),
		}
		if limit.EventsPerMinute > 0 || limit.OpensPerMinute > 0 {
			result[key] = limit
		}
	}
	return result
}

func userRateLimitsToMap(limits map[string]userRateLimit) map[string]any {
	result := map[string]any{}
	for key, limit := range limits {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = map[string]any{
			"events_per_minute": limit.EventsPerMinute,
			"opens_per_minute":  limit.OpensPerMinute,
		}
	}
	return result
}

func moderationNotesFromAny(value any) []moderationNote {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	notes := make([]moderationNote, 0, len(raw))
	for _, value := range raw {
		noteMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		note := moderationNote{
			PublicKey: strings.TrimSpace(stringFromAny(noteMap["public_key"])),
			Message:   stringFromAny(noteMap["message"]),
			CreatedBy: stringFromAny(noteMap["created_by"]),
			CreatedAt: stringFromAny(noteMap["created_at"]),
		}
		if note.PublicKey != "" || note.Message != "" {
			notes = append(notes, note)
		}
	}
	return notes
}

func moderationNotesToSlice(notes []moderationNote) []any {
	result := make([]any, 0, len(notes))
	for _, note := range notes {
		result = append(result, map[string]any{
			"public_key": note.PublicKey,
			"message":    note.Message,
			"created_by": note.CreatedBy,
			"created_at": note.CreatedAt,
		})
	}
	return result
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stringSliceFromAny(value any) []string {
	switch values := value.(type) {
	case []any:
		roles := make([]string, 0, len(values))
		for _, item := range values {
			roles = append(roles, fmt.Sprint(item))
		}
		return sortedUniqueRoles(roles)
	case []string:
		return sortedUniqueRoles(values)
	case string:
		return sortedUniqueRoles(strings.Split(values, ","))
	default:
		return nil
	}
}

func orderedStringSliceFromAny(value any) []string {
	values := []string{}
	switch raw := value.(type) {
	case []any:
		for _, item := range raw {
			values = append(values, fmt.Sprint(item))
		}
	case []string:
		values = append(values, raw...)
	case string:
		values = append(values, strings.Split(raw, ",")...)
	default:
		return nil
	}
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func sortDoorSummariesByPolicyOrder(doors []protocol.DoorSummary, order []string) {
	if len(doors) < 2 || len(order) == 0 {
		return
	}
	orderIndex := map[string]int{}
	for index, doorID := range order {
		orderIndex[doorID] = index
	}
	sort.SliceStable(doors, func(i, j int) bool {
		left, leftOK := orderIndex[doors[i].ID]
		right, rightOK := orderIndex[doors[j].ID]
		switch {
		case leftOK && rightOK:
			return left < right
		case leftOK:
			return true
		case rightOK:
			return false
		default:
			return false
		}
	})
}

func doorRolesForPolicy(policy stationPolicy, doorID string) ([]string, bool) {
	roles, ok := policy.DoorRoles[doorID]
	return roles, ok
}

func doorSummaryWithPolicy(doorSummary protocol.DoorSummary, policy stationPolicy) protocol.DoorSummary {
	roles, ok := doorRolesForPolicy(policy, doorSummary.ID)
	if ok {
		doorSummary.Visibility = visibilityPublic
		doorSummary.Access = accessRole
		doorSummary.Roles = roles
	}
	doorSummary.Disabled = policy.DisabledDoors[doorSummary.ID]
	return doorSummary
}
