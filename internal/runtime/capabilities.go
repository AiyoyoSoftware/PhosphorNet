package runtime

import (
	"fmt"
	"sort"
	"strings"
)

const (
	CapabilityStateUserRead    = "state:user:read"
	CapabilityStateUserWrite   = "state:user:write"
	CapabilityStateRoomRead    = "state:room:read"
	CapabilityStateRoomWrite   = "state:room:write"
	CapabilityStateGlobalRead  = "state:global:read"
	CapabilityStateGlobalWrite = "state:global:write"

	CapabilityBroadcastRoom = "broadcast:room"
	CapabilityBroadcastDoor = "broadcast:door"
	CapabilityBroadcastUser = "broadcast:user"

	CapabilityNotifySelf = "notify:self"
	CapabilityNotifyRoom = "notify:room"
	CapabilityNotifyDoor = "notify:door"
	CapabilityNotifyUser = "notify:user"
	CapabilityNotifyAll  = "notify:all"

	CapabilityCaptureKeys        = "capture_keys"
	CapabilityTransitionOpenDoor = "transition:open_door"

	CapabilityAdminReadStation      = "admin:read_station"
	CapabilityAdminReadUsers        = "admin:read_users"
	CapabilityAdminReadDoors        = "admin:read_doors"
	CapabilityAdminReadRuntime      = "admin:read_runtime"
	CapabilityAdminReadStorage      = "admin:read_storage"
	CapabilityAdminReadLogs         = "admin:read_logs"
	CapabilityAdminSetUserRoles     = "admin:set_user_roles"
	CapabilityAdminSetStationAccess = "admin:set_station_access"
	CapabilityAdminSetDoorPolicy    = "admin:set_door_policy"
	CapabilityAdminSetDoorSettings  = "admin:set_door_settings"
	CapabilityAdminReloadManifests  = "admin:reload_manifests"
	CapabilityAdminReorderDoors     = "admin:reorder_doors"
	CapabilityAdminSetStationNotice = "admin:set_station_notice"
	CapabilityAdminSetMaintenance   = "admin:set_maintenance"
	CapabilityAdminModerateUsers    = "admin:moderate_users"
)

const CapabilityActionPrefix = "action:"

var knownCapabilities = map[string]bool{
	CapabilityStateUserRead:         true,
	CapabilityStateUserWrite:        true,
	CapabilityStateRoomRead:         true,
	CapabilityStateRoomWrite:        true,
	CapabilityStateGlobalRead:       true,
	CapabilityStateGlobalWrite:      true,
	CapabilityBroadcastRoom:         true,
	CapabilityBroadcastDoor:         true,
	CapabilityBroadcastUser:         true,
	CapabilityNotifySelf:            true,
	CapabilityNotifyRoom:            true,
	CapabilityNotifyDoor:            true,
	CapabilityNotifyUser:            true,
	CapabilityNotifyAll:             true,
	CapabilityCaptureKeys:           true,
	CapabilityTransitionOpenDoor:    true,
	CapabilityAdminReadStation:      true,
	CapabilityAdminReadUsers:        true,
	CapabilityAdminReadDoors:        true,
	CapabilityAdminReadRuntime:      true,
	CapabilityAdminReadStorage:      true,
	CapabilityAdminReadLogs:         true,
	CapabilityAdminSetUserRoles:     true,
	CapabilityAdminSetStationAccess: true,
	CapabilityAdminSetDoorPolicy:    true,
	CapabilityAdminSetDoorSettings:  true,
	CapabilityAdminReloadManifests:  true,
	CapabilityAdminReorderDoors:     true,
	CapabilityAdminSetStationNotice: true,
	CapabilityAdminSetMaintenance:   true,
	CapabilityAdminModerateUsers:    true,
}

var deprecatedPermissionCapabilities = map[string][]string{
	"shared_state": {
		CapabilityStateRoomRead,
		CapabilityStateRoomWrite,
		CapabilityBroadcastRoom,
	},
	"raw_keys": {
		CapabilityCaptureKeys,
	},
	"global_state": {
		CapabilityStateGlobalRead,
		CapabilityStateGlobalWrite,
	},
	"maintenance": {
		CapabilityAdminSetMaintenance,
	},
	"admin": {
		CapabilityStateUserRead,
		CapabilityStateUserWrite,
		CapabilityAdminReadStation,
		CapabilityAdminReadUsers,
		CapabilityAdminReadDoors,
		CapabilityAdminReadRuntime,
		CapabilityAdminReadStorage,
		CapabilityAdminReadLogs,
		CapabilityAdminSetUserRoles,
		CapabilityAdminSetDoorPolicy,
		CapabilityAdminSetDoorSettings,
		CapabilityAdminReloadManifests,
		CapabilityAdminSetStationNotice,
		CapabilityAdminSetMaintenance,
		CapabilityAdminReorderDoors,
		CapabilityStateGlobalRead,
		CapabilityStateGlobalWrite,
		CapabilityNotifySelf,
		CapabilityNotifyAll,
		CapabilityCaptureKeys,
	},
}

func NormalizeCapabilities(capabilities []string, permissions []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(capabilities))
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		result = append(result, value)
	}
	for _, capability := range capabilities {
		add(capability)
	}
	for _, permission := range permissions {
		for _, capability := range deprecatedPermissionCapabilities[strings.ToLower(strings.TrimSpace(permission))] {
			add(capability)
		}
	}
	sort.Strings(result)
	return result
}

func ValidateCapabilities(capabilities []string) error {
	for _, capability := range capabilities {
		if strings.HasPrefix(capability, CapabilityActionPrefix) {
			if validActionRuleID(strings.TrimPrefix(capability, CapabilityActionPrefix)) {
				continue
			}
			return fmt.Errorf("invalid action capability %q", capability)
		}
		if !knownCapabilities[capability] {
			return fmt.Errorf("unknown capability %q", capability)
		}
	}
	return nil
}

func ActionCapability(ruleID string) string {
	return CapabilityActionPrefix + ruleID
}

func validActionRuleID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || index > 0 && (char == '.' || char == '_' || char == '-') {
			continue
		}
		return false
	}
	return true
}

func HasCapability(capabilities []string, capability string) bool {
	for _, candidate := range capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}
