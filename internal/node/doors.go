package node

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"phosphornet/internal/identity"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
	"phosphornet/internal/storage"
)

func (s *Server) openDoor(ctx context.Context, session *sessionState, doorID string) error {
	return s.openDoorWithBudget(ctx, session, doorID, protocol.MaxTransitionsPerResponse)
}

func (s *Server) openDoorWithBudget(ctx context.Context, session *sessionState, doorID string, transitionBudget int) error {
	return s.openDoorWithOptions(ctx, session, doorID, openDoorOptions{transitionBudget: transitionBudget})
}

func (s *Server) openInitialDoor(ctx context.Context, session *sessionState, doorID string, suppressJoin bool) error {
	return s.openDoorWithOptions(ctx, session, doorID, openDoorOptions{
		transitionBudget: protocol.MaxTransitionsPerResponse,
		initial:          true,
		suppressJoin:     suppressJoin,
	})
}

type openDoorOptions struct {
	transitionBudget int
	initial          bool
	suppressJoin     bool
}

func (s *Server) openDoorWithOptions(ctx context.Context, session *sessionState, doorID string, options openDoorOptions) error {
	if !options.initial {
		if err := s.enforceOpenRateLimit(ctx, session); err != nil {
			return err
		}
	}
	door, ok := s.findDoor(doorID)
	if !ok {
		s.events.add("access_denied", doorID, session.publicKey, "unknown door")
		s.audit(ctx, auditEvent(session.publicKey, "door.access_denied", doorID, "denied", map[string]any{"reason": "unknown door"}))
		message := fmt.Sprintf("unknown door %q", doorID)
		return protocol.NewCodedError(protocol.ErrorRuntimeDeniedPolicy, message, nil)
	}
	if !s.canAccessDoor(ctx, session, door) {
		s.events.add("access_denied", doorID, session.publicKey, "door access denied")
		s.audit(ctx, auditEvent(session.publicKey, "door.access_denied", doorID, "denied", map[string]any{"reason": "door access denied"}))
		message := fmt.Sprintf("access denied for door %q", doorID)
		return protocol.NewCodedError(protocol.ErrorRuntimeDeniedPolicy, message, nil)
	}

	if session.activeDoor != "" && session.activeDoor != door.ID {
		if activeDoor, ok := s.findDoor(session.activeDoor); ok {
			response, err := s.invokeDoorLifecycle(ctx, activeDoor, session, protocol.LifecycleOnLeave, nil)
			if err == nil {
				_ = s.applyDoorEffectsWithBudget(ctx, session, activeDoor, response, true, options.transitionBudget)
			}
		}
	}

	s.sessions.updateDoor(session.id, door.ID)
	session.activeDoor = door.ID
	session.roomID = implicitRoomID(door.ID)
	s.events.add("door_opened", door.ID, session.publicKey, "opened "+door.ID)

	if !options.suppressJoin {
		response, err := s.invokeDoorLifecycle(ctx, door, session, protocol.LifecycleOnJoin, nil)
		if err != nil {
			s.events.add("runtime_error", door.ID, session.publicKey, err.Error())
			return err
		}
		if err := s.applyDoorEffectsWithBudget(ctx, session, door, response, true, options.transitionBudget); err != nil {
			return err
		}
		if sessionMovedToDifferentDoor(session, door) {
			return nil
		}
	}
	return s.renderDoorWithBudget(ctx, session, door, options.transitionBudget)
}

func (s *Server) handleDoorEvent(ctx context.Context, session *sessionState, event protocol.UIEvent) error {
	if session.activeDoor == "" {
		return fmt.Errorf("no active door for event routing")
	}
	door, ok := s.findDoor(session.activeDoor)
	if !ok {
		return fmt.Errorf("active door %q is not registered", session.activeDoor)
	}
	if !s.canAccessDoor(ctx, session, door) {
		s.events.add("access_denied", door.ID, session.publicKey, "active door access denied")
		s.audit(ctx, auditEvent(session.publicKey, "door.access_denied", door.ID, "denied", map[string]any{"reason": "active door access denied"}))
		return fmt.Errorf("access denied for door %q", door.ID)
	}
	if err := session.validateEvent(event); err != nil {
		s.events.add("event_rejected", door.ID, session.publicKey, err.Error())
		return err
	}
	if err := s.enforceEventModeration(ctx, session, event); err != nil {
		return err
	}
	response, err := s.invokeDoorUpdate(ctx, door, session, event)
	if err != nil {
		s.events.add("runtime_error", door.ID, session.publicKey, err.Error())
		return err
	}
	if err := s.applyDoorEffectsWithBudget(ctx, session, door, response, true, protocol.MaxTransitionsPerResponse); err != nil {
		return err
	}
	if sessionMovedToDifferentDoor(session, door) {
		return nil
	}
	if responseReloadsManifests(response) {
		refreshedDoor, ok := s.findDoor(door.ID)
		if !ok {
			return fmt.Errorf("active door %q is not available after manifest reload", door.ID)
		}
		return s.renderDoor(ctx, session, refreshedDoor)
	}
	if door.ID == adminDoorID && responseUpdatesDoorSettings(response) {
		if err := s.renderSessionsWithDoorSettings(ctx, session); err != nil {
			return err
		}
		return s.renderDoor(ctx, session, door)
	}
	if responseUpdatesProfile(response) {
		return s.renderDoor(ctx, session, door)
	}
	return session.writeRender(ctx, response.View)
}

func (s *Server) renderSessionsWithDoorSettings(ctx context.Context, source *sessionState) error {
	for _, target := range s.sessions.allSessions() {
		if source != nil && target.id == source.id {
			continue
		}
		targetDoor, ok := s.findDoor(target.activeDoor)
		if !ok || len(targetDoor.Settings) == 0 {
			continue
		}
		if err := s.renderDoor(ctx, target, targetDoor); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) renderDoor(ctx context.Context, session *sessionState, door runtime.DoorManifest) error {
	return s.renderDoorWithBudget(ctx, session, door, protocol.MaxTransitionsPerResponse)
}

func (s *Server) renderDoorWithBudget(ctx context.Context, session *sessionState, door runtime.DoorManifest, transitionBudget int) error {
	response, err := s.invokeDoorView(ctx, door, session)
	if err != nil {
		s.events.add("runtime_error", door.ID, session.publicKey, err.Error())
		return err
	}
	if err := s.applyDoorEffectsWithBudget(ctx, session, door, response, false, transitionBudget); err != nil {
		return err
	}
	if sessionMovedToDifferentDoor(session, door) {
		return nil
	}
	return session.writeRender(ctx, response.View)
}

func sessionMovedToDifferentDoor(session *sessionState, door runtime.DoorManifest) bool {
	return session != nil && session.activeDoor != "" && session.activeDoor != door.ID
}

func (s *Server) invokeDoorView(parent context.Context, door runtime.DoorManifest, session *sessionState) (runtime.DoorResponse, error) {
	return s.invokeDoorLifecycle(parent, door, session, protocol.LifecycleView, nil)
}

func (s *Server) invokeDoorUpdate(parent context.Context, door runtime.DoorManifest, session *sessionState, event protocol.UIEvent) (runtime.DoorResponse, error) {
	return s.invokeDoorLifecycle(parent, door, session, protocol.LifecycleUpdate, &event)
}

func (s *Server) invokeDoorLifecycle(parent context.Context, door runtime.DoorManifest, session *sessionState, lifecycle protocol.Lifecycle, event *protocol.UIEvent) (runtime.DoorResponse, error) {
	doorsRoot, err := filepath.Abs(s.cfg.DoorsDir)
	if err != nil {
		return runtime.DoorResponse{}, fmt.Errorf("resolve doors dir: %w", err)
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	scopeIDs := storage.StateScopeIDs{
		User:   session.publicKey,
		Room:   implicitRoomID(door.ID),
		Global: "global",
	}
	state, err := s.store.LoadScopedState(ctx, door.ID, scopeIDs)
	if err != nil {
		return runtime.DoorResponse{}, protocol.NewCodedError(protocol.ErrorStorage, err.Error(), err)
	}
	state = filterStateSnapshotForDoor(door, state)
	nodeDoors := s.visibleDoorSummaries(ctx, session)
	if isPrivilegedRole(session.role) && doorHasCapability(door, runtime.CapabilityAdminReadDoors) {
		nodeDoors = s.allDoorSummaries(ctx)
	}
	policy := s.loadStationPolicy(ctx)
	presence := s.sessions.presence(scopeIDs.Room, door.ID)
	s.enrichPresence(ctx, &presence)
	allOnlineUsers := s.sessions.allPresence()
	allPresence := protocol.PresenceSnapshot{AllUsers: allOnlineUsers}
	s.enrichPresence(ctx, &allPresence)
	allOnlineUsers = allPresence.AllUsers
	presence.AllUsers = allOnlineUsers
	knownUsers := s.knownUsers(ctx, allOnlineUsers, false)
	adminContext := s.runtimeAdminContext(ctx, session, door, policy, presence.AllUsers)
	profile, err := s.store.LoadUserProfile(ctx, session.publicKey)
	if err != nil {
		return runtime.DoorResponse{}, protocol.NewCodedError(protocol.ErrorStorage, err.Error(), err)
	}
	_, muted := activeModerationEntry(policy.Moderation.MutedKeys[session.publicKey])
	runtimeCtx := protocol.RuntimeContext{
		Session: protocol.RuntimeSession{ID: session.id},
		User: protocol.RuntimeUser{
			PublicKey:   session.publicKey,
			Fingerprint: identity.Fingerprint(session.publicKey),
			Role:        session.role,
			DisplayName: profile.DisplayName,
			Bio:         profile.Bio,
			StatusLine:  profile.StatusLine,
			Guest:       profile.DisplayName == "",
		},
		Node: protocol.RuntimeNode{
			ID:              s.cfg.NodeID,
			Name:            s.cfg.Name,
			Fingerprint:     identity.Fingerprint(s.cfg.NodeID),
			AccessMode:      normalizeAccess(s.cfg.Access.Mode),
			MaintenanceMode: policy.MaintenanceMode,
			Doors:           nodeDoors,
		},
		Room: protocol.RuntimeRoom{
			ID:     scopeIDs.Room,
			DoorID: door.ID,
		},
		State:    state,
		Settings: runtime.ResolveDoorSettings(door, policy.DoorSettings[door.ID]),
		Presence: presence,
		Users:    knownUsers,
		Admin:    adminContext,
		Permissions: protocol.RuntimePermissions{
			Role:           session.role,
			Capabilities:   append([]string{}, door.Capabilities...),
			CanWriteGlobal: isPrivilegedRole(session.role) && doorHasCapability(door, runtime.CapabilityStateGlobalWrite),
			Muted:          muted,
		},
	}
	request := protocol.RuntimeRequest{
		ContractVersion: protocol.RuntimeContractVersion,
		Door:            protocol.RuntimeDoor{ID: door.ID, Name: door.Name},
		Lifecycle:       lifecycle,
		Context:         runtimeCtx,
		Event:           event,
	}
	response, err := runtime.InvokeDoorLifecycleWithOptions(ctx, doorsRoot, door, s.cfg.Runtime, request)
	if err != nil {
		return runtime.DoorResponse{}, err
	}
	if err := validateResponseCapabilities(session, door, response); err != nil {
		s.audit(ctx, auditEvent(session.publicKey, "effect.denied", door.ID, "denied", map[string]any{
			"reason":    err.Error(),
			"lifecycle": string(lifecycle),
		}))
		return runtime.DoorResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeDeniedPolicy, err.Error(), err)
	}
	if err := s.store.ApplyStateOps(ctx, door.ID, scopeIDs, session.role, response.StateOps); err != nil {
		return runtime.DoorResponse{}, protocol.NewCodedError(protocol.ErrorStorage, err.Error(), err)
	}
	return response, nil
}

func (s *Server) enrichPresence(ctx context.Context, presence *protocol.PresenceSnapshot) {
	if s.store == nil || presence == nil {
		return
	}
	keys := make([]string, 0, len(presence.RoomUsers)+len(presence.DoorUsers)+len(presence.AllUsers))
	for _, user := range presence.RoomUsers {
		keys = append(keys, user.PublicKey)
	}
	for _, user := range presence.DoorUsers {
		keys = append(keys, user.PublicKey)
	}
	for _, user := range presence.AllUsers {
		keys = append(keys, user.PublicKey)
	}
	profiles, err := s.store.LoadUserProfiles(ctx, keys)
	if err != nil {
		return
	}
	apply := func(users []protocol.PresenceUser) {
		for i := range users {
			profile := profiles[users[i].PublicKey]
			users[i].DisplayName = profile.DisplayName
			users[i].StatusLine = profile.StatusLine
			users[i].Guest = profile.DisplayName == ""
		}
	}
	apply(presence.RoomUsers)
	apply(presence.DoorUsers)
	apply(presence.AllUsers)
}

func (s *Server) knownUsers(ctx context.Context, onlineUsers []protocol.PresenceUser, includeOperationalDetails bool) []protocol.KnownUser {
	if s.store == nil {
		return nil
	}
	records, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil
	}
	online := map[string]bool{}
	for _, user := range onlineUsers {
		online[user.PublicKey] = true
	}
	users := make([]protocol.KnownUser, 0, len(records))
	for _, record := range records {
		role := s.roleForPublicKey(ctx, record.PublicKey)
		user := protocol.KnownUser{
			PublicKey:   record.PublicKey,
			Fingerprint: identity.Fingerprint(record.PublicKey),
			Role:        role,
			DisplayName: record.Name,
			Bio:         record.Bio,
			StatusLine:  record.StatusLine,
			Guest:       record.Name == "",
			Online:      online[record.PublicKey],
		}
		if includeOperationalDetails {
			user.FirstSeen = record.FirstSeen
			user.LastSeen = record.LastSeen
		}
		users = append(users, user)
	}
	return users
}

func (s *Server) runtimeStorage(ctx context.Context) protocol.RuntimeStorage {
	snapshot := protocol.RuntimeStorage{DatabasePath: s.cfg.Database}
	if s.store == nil {
		return snapshot
	}
	records, err := s.store.ListStateRecords(ctx)
	if err != nil {
		return snapshot
	}
	snapshot.StateRecords = make([]protocol.StateRecordSummary, 0, len(records))
	for _, record := range records {
		snapshot.StateRecords = append(snapshot.StateRecords, protocol.StateRecordSummary{
			DoorID:    record.DoorID,
			Scope:     record.Scope,
			ScopeID:   record.ScopeID,
			Bytes:     record.Bytes,
			UpdatedAt: record.UpdatedAt,
		})
	}
	return snapshot
}

func filterStateSnapshotForDoor(door runtime.DoorManifest, state protocol.RuntimeStateSnapshot) protocol.RuntimeStateSnapshot {
	if !doorHasCapability(door, runtime.CapabilityStateUserRead) {
		state.User = map[string]any{}
	}
	if !doorHasCapability(door, runtime.CapabilityStateRoomRead) {
		state.Room = map[string]any{}
	}
	if !doorHasCapability(door, runtime.CapabilityStateGlobalRead) {
		state.Global = map[string]any{}
	}
	return state
}

func (s *Server) runtimeAdminContext(ctx context.Context, session *sessionState, door runtime.DoorManifest, policy stationPolicy, onlineUsers []protocol.PresenceUser) *protocol.RuntimeAdmin {
	if session == nil || !isPrivilegedRole(session.role) {
		return nil
	}
	admin := protocol.RuntimeAdmin{}
	if doorHasCapability(door, runtime.CapabilityAdminReadStation) {
		admin.StationAllowlist = append([]string{}, s.cfg.Access.Allowlist...)
		admin.Admins = append([]string{}, s.cfg.Access.Admins...)
		admin.Policy = stationPolicyToMap(policy)
	}
	if doorHasCapability(door, runtime.CapabilityAdminReadUsers) {
		admin.Users = s.knownUsers(ctx, onlineUsers, true)
	}
	if doorHasCapability(door, runtime.CapabilityAdminReadDoors) {
		admin.Doors = s.allDoorSummaries(ctx)
	}
	if doorHasCapability(door, runtime.CapabilityAdminReadStorage) {
		admin.Storage = s.runtimeStorage(ctx)
		admin.DatabasePath = s.cfg.Database
	}
	if doorHasCapability(door, runtime.CapabilityAdminReadRuntime) {
		admin.DoorsDir = s.cfg.DoorsDir
		admin.DefaultRuntime = s.cfg.Runtime.DefaultRuntime
		admin.LuaSandbox = protocol.RuntimeLua{
			Profile:        runtime.NormalizeSandboxProfileForDisplay(s.cfg.Runtime.Lua.Profile),
			Libraries:      s.cfg.Runtime.Lua.Libraries,
			MaxMemoryKB:    s.cfg.Runtime.Lua.MaxMemoryKB,
			MaxExecutionMS: s.cfg.Runtime.Lua.MaxExecutionMS,
		}
	}
	if doorHasCapability(door, runtime.CapabilityAdminReadLogs) {
		admin.Events = s.events.recent()
	}
	if admin.Policy == nil && admin.Users == nil && admin.Doors == nil && len(admin.Storage.StateRecords) == 0 && admin.DatabasePath == "" && admin.DoorsDir == "" && admin.DefaultRuntime == "" && len(admin.Events) == 0 {
		return nil
	}
	return &admin
}
