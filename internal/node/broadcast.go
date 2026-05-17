package node

import (
	"context"
	"fmt"

	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
)

func (s *Server) applyDoorEffects(ctx context.Context, source *sessionState, door runtime.DoorManifest, response runtime.DoorResponse, broadcastRenders bool) error {
	return s.applyDoorEffectsWithBudget(ctx, source, door, response, broadcastRenders, protocol.MaxTransitionsPerResponse)
}

func (s *Server) applyDoorEffectsWithBudget(ctx context.Context, source *sessionState, door runtime.DoorManifest, response runtime.DoorResponse, broadcastRenders bool, transitionBudget int) error {
	if len(response.Transitions) > protocol.MaxTransitionsPerResponse {
		return fmt.Errorf("too many transitions in one response (%d > %d)", len(response.Transitions), protocol.MaxTransitionsPerResponse)
	}
	for _, update := range response.ProfileUpdates {
		if source == nil || s.store == nil {
			continue
		}
		profile, err := s.store.SaveUserProfile(ctx, source.publicKey, update)
		if err != nil {
			if writeErr := source.write(ctx, protocol.NotifyMessage{
				Type:    protocol.TypeNotify,
				Level:   "warning",
				Message: err.Error(),
			}); writeErr != nil {
				return writeErr
			}
			continue
		}
		name := profile.DisplayName
		if name == "" {
			name = profile.PublicKey
		}
		s.events.add("profile_updated", door.ID, source.publicKey, "updated station profile for "+name)
	}

	adminResult, err := s.applyAdminOps(ctx, source, door, response.AdminOps)
	if err != nil {
		return err
	}

	if responseUpdatesStationPolicy(response) || adminResult.policyChanged {
		policy := s.loadStationPolicy(ctx)
		s.sessions.refreshRoles(func(publicKey string) string {
			return s.roleForPublicKeyWithPolicy(publicKey, policy)
		})
		for _, target := range s.sessions.allSessions() {
			if err := s.sendDoorList(ctx, target); err != nil {
				return err
			}
		}
	}

	for _, notify := range response.Notifies {
		for _, target := range s.sessions.notifyTargets(source, door.ID, notify) {
			if source != nil && target.id == source.id && notify.Target != "" && notify.Target != protocol.NotifyTargetSelf {
				continue
			}
			if err := target.write(ctx, protocol.NotifyMessage{
				Type:    protocol.TypeNotify,
				Level:   notify.Level,
				Message: notify.Message,
			}); err != nil {
				return err
			}
		}
	}

	for _, transition := range response.Transitions {
		if transition.Kind != protocol.TransitionKindOpenDoor {
			continue
		}
		targetDoorID := transition.DoorID
		if targetDoorID == "" || source == nil {
			continue
		}
		if source.activeDoor == targetDoorID {
			continue
		}
		if transitionBudget <= 0 {
			return fmt.Errorf("transition budget exhausted")
		}
		return s.openDoorWithBudget(ctx, source, targetDoorID, transitionBudget-1)
	}

	if responseUpdatesProfile(response) {
		for _, target := range s.sessions.allSessions() {
			if source != nil && target.id == source.id {
				continue
			}
			targetDoor, ok := s.findDoor(target.activeDoor)
			if !ok {
				continue
			}
			if err := s.renderDoorWithBudget(ctx, target, targetDoor, protocol.MaxTransitionsPerResponse); err != nil {
				return err
			}
		}
	}

	if !broadcastRenders {
		return nil
	}
	for _, broadcast := range response.Broadcasts {
		for _, target := range s.sessions.targets(source, door.ID, broadcast) {
			if target.id == source.id {
				continue
			}
			targetDoor, ok := s.findDoor(target.activeDoor)
			if !ok {
				continue
			}
			if err := s.renderDoorWithBudget(ctx, target, targetDoor, protocol.MaxTransitionsPerResponse); err != nil {
				return err
			}
		}
	}
	return nil
}

func responseUpdatesStationPolicy(response runtime.DoorResponse) bool {
	for _, op := range response.AdminOps {
		switch op.Op {
		case "set_user_role", "set_door_roles", "set_door_enabled", "reorder_doors", "set_maintenance", "record_maintenance_checkpoint", "reset_maintenance", "set_station_notice", "clear_station_notices", "ban_key", "unban_key", "mute_key", "unmute_key", "set_user_rate_limit", "record_moderation_note":
			return true
		}
	}
	return false
}

func responseUpdatesDoorSettings(response runtime.DoorResponse) bool {
	for _, op := range response.AdminOps {
		if op.Op == "set_door_setting" {
			return true
		}
	}
	return false
}

func responseReloadsManifests(response runtime.DoorResponse) bool {
	for _, op := range response.AdminOps {
		if op.Op == "reload_manifests" {
			return true
		}
	}
	return false
}

func responseUpdatesProfile(response runtime.DoorResponse) bool {
	return len(response.ProfileUpdates) > 0
}
