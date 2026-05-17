package node

import (
	"context"
	"testing"

	"phosphornet/internal/config"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
)

func TestValidateEventAgainstPolicyRejectsForgedTarget(t *testing.T) {
	policy, err := buildRenderEventPolicy(protocol.Screen(
		protocol.Button("send", "Send", "send_message"),
	))
	if err != nil {
		t.Fatalf("buildRenderEventPolicy() error = %v", err)
	}

	err = validateEventAgainstPolicy(protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "admin-delete",
		Action: "destroy_everything",
	}, policy)
	if err == nil {
		t.Fatal("validateEventAgainstPolicy() error = nil, want forged target rejection")
	}
}

func TestValidateEventAgainstPolicyRejectsKeyEventsWithoutCapture(t *testing.T) {
	policy, err := buildRenderEventPolicy(protocol.Screen(
		protocol.Text("plain screen"),
	))
	if err != nil {
		t.Fatalf("buildRenderEventPolicy() error = %v", err)
	}

	err = validateEventAgainstPolicy(protocol.UIEvent{
		Kind: protocol.EventKindKey,
		Key:  "q",
	}, policy)
	if err == nil {
		t.Fatal("validateEventAgainstPolicy() error = nil, want key-capture rejection")
	}
}

func TestBuildRenderEventPolicyRejectsDuplicateInteractiveIDs(t *testing.T) {
	_, err := buildRenderEventPolicy(protocol.Screen(
		protocol.Button("dup", "One", "first"),
		protocol.Button("dup", "Two", "second"),
	))
	if err == nil {
		t.Fatal("buildRenderEventPolicy() error = nil, want duplicate id rejection")
	}
}

func TestRouteClientMessageRejectsMismatchedSessionID(t *testing.T) {
	server := newServer(config.DefaultNodeConfig(), nil, nil)
	session := &sessionState{id: "session-1"}

	err := server.routeClientMessage(context.Background(), session, protocol.EventMessage{
		Type:      protocol.TypeEvent,
		SessionID: "session-2",
		Event:     protocol.UIEvent{Kind: protocol.EventKindKey, Key: "q"},
	})
	if err == nil {
		t.Fatal("routeClientMessage() error = nil, want mismatched session rejection")
	}
}

func TestApplyDoorEffectsIgnoresSameDoorTransition(t *testing.T) {
	server := newServer(config.DefaultNodeConfig(), nil, nil)
	session := &sessionState{id: "s1", activeDoor: "chat"}

	err := server.applyDoorEffects(context.Background(), session, runtime.DoorManifest{ID: "chat"}, runtime.DoorResponse{
		Transitions: []protocol.TransitionEffect{{Kind: protocol.TransitionKindOpenDoor, DoorID: "chat"}},
	}, false)
	if err != nil {
		t.Fatalf("applyDoorEffects() error = %v", err)
	}
}

func TestApplyDoorEffectsRejectsTooManyTransitions(t *testing.T) {
	server := newServer(config.DefaultNodeConfig(), nil, nil)
	session := &sessionState{id: "s1", activeDoor: "chat"}
	transitions := make([]protocol.TransitionEffect, 0, protocol.MaxTransitionsPerResponse+1)
	for i := 0; i < protocol.MaxTransitionsPerResponse+1; i++ {
		transitions = append(transitions, protocol.TransitionEffect{Kind: protocol.TransitionKindOpenDoor, DoorID: "chat"})
	}

	err := server.applyDoorEffects(context.Background(), session, runtime.DoorManifest{ID: "chat"}, runtime.DoorResponse{
		Transitions: transitions,
	}, false)
	if err == nil {
		t.Fatal("applyDoorEffects() error = nil, want transition limit rejection")
	}
}
