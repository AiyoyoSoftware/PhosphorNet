package node

import (
	"testing"

	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
)

func TestSessionRegistryPresenceByRoomAndDoor(t *testing.T) {
	registry := newSessionRegistry()
	registry.add(&sessionState{id: "one", publicKey: "user-one", role: "member", activeDoor: "chat", roomID: "door:chat"})
	registry.add(&sessionState{id: "two", publicKey: "user-two", role: "member", activeDoor: "chat", roomID: "door:chat"})
	registry.add(&sessionState{id: "three", publicKey: "user-three", role: "member", activeDoor: "lobby", roomID: "door:lobby"})

	presence := registry.presence("door:chat", "chat")
	if len(presence.RoomUsers) != 2 {
		t.Fatalf("len(presence.RoomUsers) = %d, want 2", len(presence.RoomUsers))
	}
	if len(presence.DoorUsers) != 2 {
		t.Fatalf("len(presence.DoorUsers) = %d, want 2", len(presence.DoorUsers))
	}
}

func TestSessionRegistryBroadcastTargetsRoom(t *testing.T) {
	registry := newSessionRegistry()
	source := &sessionState{id: "one", publicKey: "user-one", role: "member", activeDoor: "chat", roomID: "door:chat"}
	registry.add(source)
	registry.add(&sessionState{id: "two", publicKey: "user-two", role: "member", activeDoor: "chat", roomID: "door:chat"})
	registry.add(&sessionState{id: "three", publicKey: "user-three", role: "member", activeDoor: "lobby", roomID: "door:lobby"})

	targets := registry.targets(source, "chat", protocol.BroadcastEffect{Scope: protocol.BroadcastScopeRoom})
	if len(targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2", len(targets))
	}
}

func TestSessionRegistryNotifyTargetsAll(t *testing.T) {
	registry := newSessionRegistry()
	source := &sessionState{id: "one", publicKey: "user-one", role: "member", activeDoor: "chat", roomID: "door:chat"}
	registry.add(source)
	registry.add(&sessionState{id: "two", publicKey: "user-two", role: "member", activeDoor: "lobby", roomID: "door:lobby"})

	targets := registry.notifyTargets(source, "admin", protocol.NotifyEffect{Target: protocol.NotifyTargetAll})
	if len(targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2", len(targets))
	}
}

func TestResponseUpdatesStationPolicy(t *testing.T) {
	if !responseUpdatesStationPolicy(runtime.DoorResponse{
		AdminOps: []protocol.AdminOp{{Op: "set_user_role"}},
	}) {
		t.Fatal("responseUpdatesStationPolicy() = false, want true for roles update")
	}
	if responseUpdatesStationPolicy(runtime.DoorResponse{
		StateOps: []protocol.StateOp{{Scope: protocol.StateScopeUser, Key: "roles"}},
	}) {
		t.Fatal("responseUpdatesStationPolicy() = true, want false for user state")
	}
}

func TestSessionRegistryRefreshRoles(t *testing.T) {
	registry := newSessionRegistry()
	registry.add(&sessionState{id: "one", publicKey: "user-one", role: "member"})
	registry.add(&sessionState{id: "two", publicKey: "user-two", role: "member"})

	registry.refreshRoles(func(publicKey string) string {
		if publicKey == "user-two" {
			return "moderator"
		}
		return "member"
	})

	sessions := registry.allSessions()
	roles := map[string]string{}
	for _, session := range sessions {
		roles[session.publicKey] = session.role
	}
	if roles["user-one"] != "member" || roles["user-two"] != "moderator" {
		t.Fatalf("roles = %#v, want user-two promoted to moderator", roles)
	}
}
