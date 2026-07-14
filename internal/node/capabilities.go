package node

import (
	"fmt"

	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
)

func doorHasCapability(door runtime.DoorManifest, capability string) bool {
	return runtime.HasCapability(door.Capabilities, capability)
}

func requireDoorCapability(door runtime.DoorManifest, capability string) error {
	if doorHasCapability(door, capability) {
		return nil
	}
	return fmt.Errorf("door %q lacks required capability %q", door.ID, capability)
}

func requirePrivilegedAdminCapability(session *sessionState, door runtime.DoorManifest, capability string) error {
	if session == nil || !isPrivilegedRole(session.role) {
		return fmt.Errorf("admin operation requires admin or sysop role")
	}
	return requireDoorCapability(door, capability)
}

func stateWriteCapability(scope protocol.StateScope) (string, error) {
	switch scope {
	case protocol.StateScopeUser:
		return runtime.CapabilityStateUserWrite, nil
	case protocol.StateScopeRoom:
		return runtime.CapabilityStateRoomWrite, nil
	case protocol.StateScopeGlobal:
		return runtime.CapabilityStateGlobalWrite, nil
	default:
		return "", fmt.Errorf("unknown state scope %q", scope)
	}
}

func broadcastCapability(scope protocol.BroadcastScope) (string, error) {
	switch scope {
	case "", protocol.BroadcastScopeRoom:
		return runtime.CapabilityBroadcastRoom, nil
	case protocol.BroadcastScopeDoor:
		return runtime.CapabilityBroadcastDoor, nil
	case protocol.BroadcastScopeUser:
		return runtime.CapabilityBroadcastUser, nil
	default:
		return "", fmt.Errorf("unknown broadcast scope %q", scope)
	}
}

func notifyCapability(target protocol.NotifyTarget) (string, error) {
	switch target {
	case "", protocol.NotifyTargetSelf:
		return runtime.CapabilityNotifySelf, nil
	case protocol.NotifyTargetRoom:
		return runtime.CapabilityNotifyRoom, nil
	case protocol.NotifyTargetDoor:
		return runtime.CapabilityNotifyDoor, nil
	case protocol.NotifyTargetUser:
		return runtime.CapabilityNotifyUser, nil
	case protocol.NotifyTargetAll:
		return runtime.CapabilityNotifyAll, nil
	default:
		return "", fmt.Errorf("unknown notify target %q", target)
	}
}

func validateStateOpCapabilities(session *sessionState, door runtime.DoorManifest, ops []protocol.StateOp) error {
	for _, op := range ops {
		capability, err := stateWriteCapability(op.Scope)
		if err != nil {
			return err
		}
		if err := requireDoorCapability(door, capability); err != nil {
			return err
		}
		if op.Scope == protocol.StateScopeGlobal && (session == nil || !isPrivilegedRole(session.role)) {
			return fmt.Errorf("global state writes require admin or sysop role")
		}
	}
	return nil
}

func validateResponseCapabilities(session *sessionState, door runtime.DoorManifest, response runtime.DoorResponse) error {
	if err := validateStateOpCapabilities(session, door, response.StateOps); err != nil {
		return err
	}
	for _, broadcast := range response.Broadcasts {
		capability, err := broadcastCapability(broadcast.Scope)
		if err != nil {
			return err
		}
		if err := requireDoorCapability(door, capability); err != nil {
			return err
		}
	}
	for _, notify := range response.Notifies {
		capability, err := notifyCapability(notify.Target)
		if err != nil {
			return err
		}
		if err := requireDoorCapability(door, capability); err != nil {
			return err
		}
	}
	for _, transition := range response.Transitions {
		if transition.Kind != protocol.TransitionKindOpenDoor {
			continue
		}
		if err := requireDoorCapability(door, runtime.CapabilityTransitionOpenDoor); err != nil {
			return err
		}
	}
	for _, action := range response.Actions {
		if err := requireDoorCapability(door, runtime.ActionCapability(action.RuleID)); err != nil {
			return err
		}
	}
	if response.View.CaptureKeys {
		if err := requireDoorCapability(door, runtime.CapabilityCaptureKeys); err != nil {
			return err
		}
	}
	if err := validateAdminOps(session, door, response.AdminOps); err != nil {
		return err
	}
	return nil
}
