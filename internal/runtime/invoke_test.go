package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"phosphornet/internal/protocol"
)

func TestInvokeDoorView(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:    "lobby",
		Name:  "Lobby",
		Entry: "app.lua",
	}, protocol.RuntimeContext{
		Session: protocol.RuntimeSession{ID: "test-session"},
		User: protocol.RuntimeUser{
			PublicKey:   "ed25519:test",
			Fingerprint: "test",
			Role:        "member",
		},
		Node: protocol.RuntimeNode{
			ID:   "node:test",
			Name: "Test Node",
		},
		Room: protocol.RuntimeRoom{
			ID:     "door:lobby",
			DoorID: "lobby",
		},
		State: protocol.RuntimeStateSnapshot{
			User:   map[string]any{},
			Room:   map[string]any{},
			Global: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if response.View.Component != "screen" {
		t.Fatalf("response.View.Component = %q, want %q", response.View.Component, "screen")
	}
}

func TestInvokeDoorUpdateReturnsStateOps(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := protocol.RuntimeContext{
		Session: protocol.RuntimeSession{ID: "test-session"},
		User: protocol.RuntimeUser{
			PublicKey:   "ed25519:test",
			Fingerprint: "test",
			Role:        "member",
		},
		Node: protocol.RuntimeNode{
			ID:   "node:test",
			Name: "Test Node",
		},
		Room: protocol.RuntimeRoom{
			ID:     "door:lobby",
			DoorID: "lobby",
		},
		State: protocol.RuntimeStateSnapshot{
			User:   map[string]any{},
			Room:   map[string]any{},
			Global: map[string]any{},
		},
	}

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:    "lobby",
		Name:  "Lobby",
		Entry: "app.lua",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "lobby-actions",
		Action: "record_visit",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate() error = %v", err)
	}
	if len(response.StateOps) != 1 {
		t.Fatalf("len(response.StateOps) = %d, want 1", len(response.StateOps))
	}
	if response.StateOps[0].Scope != protocol.StateScopeUser {
		t.Fatalf("response.StateOps[0].Scope = %q, want %q", response.StateOps[0].Scope, protocol.StateScopeUser)
	}
	if response.StateOps[0].Op != protocol.StateOpSet {
		t.Fatalf("response.StateOps[0].Op = %q, want %q", response.StateOps[0].Op, protocol.StateOpSet)
	}
}

func TestInvokeChatDoorView(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:      "chat",
		Name:    "Chat",
		Entry:   "app.lua",
		Runtime: "lua",
	}, testRuntimeContext("chat"))
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if response.View.Component != "screen" {
		t.Fatalf("response.View.Component = %q, want %q", response.View.Component, "screen")
	}
	if response.View.Scroll != "bottom" {
		t.Fatalf("response.View.Scroll = %q, want bottom", response.View.Scroll)
	}
	if len(response.View.Children) == 0 || response.View.Children[len(response.View.Children)-1].Component != "input" {
		t.Fatalf("chat view children = %#v, want input at bottom", response.View.Children)
	}
	if response.View.Children[len(response.View.Children)-1].ID != "chat-message" {
		t.Fatalf("bottom component = %#v, want chat-message input", response.View.Children[len(response.View.Children)-1])
	}
	if response.View.Children[len(response.View.Children)-1].Dock != "bottom" {
		t.Fatalf("bottom component dock = %q, want bottom", response.View.Children[len(response.View.Children)-1].Dock)
	}
	for _, child := range response.View.Children {
		if child.ID == "send-ping" {
			t.Fatalf("chat view children = %#v, want no ping button", response.View.Children)
		}
	}
}

func TestChatDoorSlashCommands(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manifest := DoorManifest{
		ID:      "chat",
		Name:    "Chat",
		Entry:   "app.lua",
		Runtime: "lua",
	}
	runtimeCtx := testRuntimeContext("chat")
	runtimeCtx.User.PublicKey = "ed25519:alice"
	runtimeCtx.User.Fingerprint = "ALICE"
	runtimeCtx.Presence.RoomUsers = []protocol.PresenceUser{
		{PublicKey: "ed25519:alice", Fingerprint: "ALICE", Role: "member"},
		{PublicKey: "ed25519:bob", Fingerprint: "BOB", Role: "member"},
	}

	response, err := InvokeDoorUpdate(ctx, doorsRoot, manifest, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindSubmit,
		Target: "chat-message",
		Values: map[string]string{"chat-message": "/nickname ada"},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(/nickname) error = %v", err)
	}
	if len(response.ProfileUpdates) != 1 {
		t.Fatalf("/nickname profile updates = %#v, want one display-name update", response.ProfileUpdates)
	}
	if response.ProfileUpdates[0].DisplayName == nil || *response.ProfileUpdates[0].DisplayName != "ada" {
		t.Fatalf("/nickname profile update = %#v, want display name ada", response.ProfileUpdates[0])
	}
	if !uiTreeContainsText(response.View, "station display name set to ada") {
		t.Fatalf("/nickname view = %#v, want local confirmation", response.View)
	}

	runtimeCtx.User.DisplayName = "ada"
	runtimeCtx.Presence.RoomUsers = []protocol.PresenceUser{
		{PublicKey: "ed25519:alice", Fingerprint: "ALICE", Role: "member", DisplayName: "ada"},
		{PublicKey: "ed25519:bob", Fingerprint: "BOB", Role: "member", DisplayName: "bob"},
	}
	runtimeCtx.Presence.AllUsers = runtimeCtx.Presence.RoomUsers
	response, err = InvokeDoorUpdate(ctx, doorsRoot, manifest, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindSubmit,
		Target: "chat-message",
		Values: map[string]string{"chat-message": "/tell bob hello there"},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(/tell) error = %v", err)
	}
	if len(response.Notifies) != 1 {
		t.Fatalf("/tell notifies = %#v, want one private notify", response.Notifies)
	}
	if response.Notifies[0].Target != protocol.NotifyTargetUser || response.Notifies[0].UserPublicKey != "ed25519:bob" {
		t.Fatalf("/tell notify = %#v, want target bob", response.Notifies[0])
	}
	if stateOpsContainKey(response.StateOps, "local_notices") {
		t.Fatalf("/tell ops = %#v, want no persisted local notices", response.StateOps)
	}
	if !uiTreeContainsText(response.View, "-> bob: hello there") {
		t.Fatalf("/tell view = %#v, want local tell confirmation", response.View)
	}

	response, err = InvokeDoorUpdate(ctx, doorsRoot, manifest, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindSubmit,
		Target: "chat-message",
		Values: map[string]string{"chat-message": "/help"},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(/help) error = %v", err)
	}
	if stateOpsContainKey(response.StateOps, "local_notices") {
		t.Fatalf("/help ops = %#v, want no persisted local help notices", response.StateOps)
	}
	if !uiTreeContainsText(response.View, "/nickname <name>") {
		t.Fatalf("/help view = %#v, want local help text", response.View)
	}
}

func TestChatDoorCapsRenderedBacklogToUIContractLimit(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("chat")
	messages := make([]any, 0, 70)
	for index := 0; index < 70; index++ {
		messages = append(messages, map[string]any{
			"from_display_name": "tester",
			"text":              "message",
		})
	}
	runtimeCtx.State.Room = map[string]any{"messages": messages}

	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:      "chat",
		Name:    "Chat",
		Entry:   "app.lua",
		Runtime: "lua",
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if len(response.View.Children) < 3 {
		t.Fatalf("chat view children = %#v, want chat log", response.View.Children)
	}
	log := response.View.Children[2]
	if log.ID != "chat-log" {
		t.Fatalf("chat log node = %#v, want chat-log", log)
	}
	if len(log.Children) != protocol.MaxUIChildren {
		t.Fatalf("len(chat log children) = %d, want %d", len(log.Children), protocol.MaxUIChildren)
	}
	if !strings.Contains(log.Children[0].Text, "older entries hidden") {
		t.Fatalf("first chat log line = %q, want truncation marker", log.Children[0].Text)
	}
}

func TestForumDoorSeedsWelcomeThreadAndRendersMarkdown(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}, testRuntimeContext("forum"))
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if response.View.Component != "screen" {
		t.Fatalf("response.View.Component = %q, want screen", response.View.Component)
	}
	if !uiTreeContainsText(response.View, "Welcome to PhosphorNet") {
		t.Fatalf("forum view = %#v, want seeded welcome thread", response.View)
	}
	if !uiTreeContainsComponent(response.View, "menu") {
		t.Fatalf("forum view = %#v, want thread list menu", response.View)
	}
	if !uiTreeContainsText(response.View, "New Thread") {
		t.Fatalf("forum view = %#v, want new thread button", response.View)
	}
}

func TestForumDoorCreateThreadAndReply(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manifest := DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}
	runtimeCtx := testRuntimeContext("forum")

	createResponse, err := InvokeDoorUpdate(ctx, doorsRoot, manifest, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "forum-create-thread",
		Action: "create_thread",
		Values: map[string]string{
			"forum-thread-title": "A New Thread",
			"forum-thread-body":  "# Hello\n\nThis is the first post.",
		},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(create_thread) error = %v", err)
	}
	if !uiTreeContainsText(createResponse.View, "A New Thread") {
		t.Fatalf("create thread view = %#v, want new thread title", createResponse.View)
	}
	if !uiTreeContainsText(createResponse.View, "guest-test") {
		t.Fatalf("create thread view = %#v, want starter author", createResponse.View)
	}
	if !uiTreeContainsText(createResponse.View, "test") {
		t.Fatalf("create thread view = %#v, want starter fingerprint", createResponse.View)
	}
	if !uiTreeContainsComponent(createResponse.View, "markdown") {
		t.Fatalf("create thread view = %#v, want markdown body", createResponse.View)
	}
	if op, ok := stateOpForKey(createResponse.StateOps, "threads"); !ok || op.Scope != protocol.StateScopeRoom {
		t.Fatalf("create thread state ops = %#v, want room threads update", createResponse.StateOps)
	}

	runtimeCtx.State.Room = map[string]any{
		"threads": []any{
			map[string]any{
				"id":              1,
				"title":           "Seeded Thread",
				"pinned":          true,
				"starter_post_id": 1,
				"created_seq":     1,
				"updated_seq":     1,
			},
		},
		"posts": []any{
			map[string]any{
				"id":          1,
				"thread_id":   1,
				"author":      "station",
				"body":        "Seed body",
				"created_seq": 1,
				"hidden":      false,
			},
		},
		"next_thread_id":    2,
		"next_post_id":      2,
		"next_activity_seq": 2,
	}
	runtimeCtx.State.User = map[string]any{"__nav_stack": []any{"thread:1"}}
	replyResponse, err := InvokeDoorUpdate(ctx, doorsRoot, manifest, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "forum-post-reply",
		Action: "post_reply",
		Values: map[string]string{
			"forum-reply-body": "A reply with **markdown**.",
		},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(post_reply) error = %v", err)
	}
	if !uiTreeContainsText(replyResponse.View, "A reply with") {
		t.Fatalf("reply view = %#v, want reply body", replyResponse.View)
	}
	if !uiTreeContainsText(replyResponse.View, "guest-test") {
		t.Fatalf("reply view = %#v, want reply author", replyResponse.View)
	}
	if !uiTreeContainsComponent(replyResponse.View, "markdown") {
		t.Fatalf("reply view = %#v, want markdown reply body", replyResponse.View)
	}
}

func TestForumOnJoinResetsToHomeView(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("forum")
	runtimeCtx.State.User = map[string]any{"__nav_stack": []any{"thread:1"}}

	response, err := InvokeDoorHook(ctx, doorsRoot, DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}, protocol.LifecycleOnJoin, runtimeCtx)
	if err != nil {
		t.Fatalf("InvokeDoorHook(on_join) error = %v", err)
	}
	if !uiTreeContainsComponent(response.View, "menu") {
		t.Fatalf("on_join view = %#v, want thread list menu", response.View)
	}
	if uiTreeContainsComponent(response.View, "button") && uiTreeContainsText(response.View, "Reply") {
		t.Fatalf("on_join view = %#v, want no thread page actions", response.View)
	}
}

func TestForumModerationControlsAreAdminOnly(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	memberCtx := testRuntimeContext("forum")
	memberCtx.State.User = map[string]any{"__nav_stack": []any{"thread:1"}}
	memberResponse, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}, memberCtx)
	if err != nil {
		t.Fatalf("InvokeDoorView(member) error = %v", err)
	}
	if uiTreeContainsText(memberResponse.View, "Moderation") {
		t.Fatalf("member view = %#v, want no moderation controls", memberResponse.View)
	}

	adminCtx := testRuntimeContext("forum")
	adminCtx.User.Role = "admin"
	adminCtx.State.User = map[string]any{"__nav_stack": []any{"thread:1"}}
	adminResponse, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}, adminCtx)
	if err != nil {
		t.Fatalf("InvokeDoorView(admin) error = %v", err)
	}
	if !uiTreeContainsText(adminResponse.View, "Moderation") {
		t.Fatalf("admin view = %#v, want moderation controls", adminResponse.View)
	}
	if !uiTreeContainsText(adminResponse.View, "Hide") || !uiTreeContainsText(adminResponse.View, "Delete") {
		t.Fatalf("admin view = %#v, want hide/delete controls", adminResponse.View)
	}
}

func TestInvokeAdminDoorUpdateReturnsAdminOps(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("admin")
	runtimeCtx.User.Role = "admin"

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "checkpoint",
		Action: "checkpoint",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate() error = %v", err)
	}
	if len(response.AdminOps) == 0 {
		t.Fatal("len(response.AdminOps) = 0, want admin ops")
	}
	if response.AdminOps[0].Op != "set_station_notice" && response.AdminOps[0].Op != "record_maintenance_checkpoint" {
		t.Fatalf("admin ops = %#v, want maintenance checkpoint/notice ops", response.AdminOps)
	}
}

func TestInvokeAdminDoorStationNoticeTargetsAll(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("admin")
	runtimeCtx.User.Role = "admin"
	runtimeCtx.State.User = map[string]any{"station_notice": "maintenance soon"}

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "send-notice",
		Action: "send_notice",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate() error = %v", err)
	}
	if len(response.Notifies) != 1 {
		t.Fatalf("len(response.Notifies) = %d, want 1", len(response.Notifies))
	}
	if response.Notifies[0].Target != protocol.NotifyTargetAll {
		t.Fatalf("notify target = %q, want all", response.Notifies[0].Target)
	}
}

func TestInvokeAdminDoorPanelsHaveGradientBackgrounds(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pages := []string{
		"home",
		"doors",
		"doors/detail:chat",
		"settings",
		"settings/detail:lobby",
		"users",
		"users/detail:test",
		"access",
		"moderation",
		"storage",
		"storage/door:chat",
		"storage/door:chat/scope:user",
		"runtime",
		"logs",
		"maintenance",
		"confirm:reset_maintenance",
	}

	for _, page := range pages {
		t.Run(page, func(t *testing.T) {
			runtimeCtx := testRuntimeContext("admin")
			runtimeCtx.User.Role = "admin"
			runtimeCtx.State.User = map[string]any{"__nav_stack": []any{page}}
			if page == "home" {
				runtimeCtx.State.User = map[string]any{}
			}
			runtimeCtx.Node.Doors = []protocol.DoorSummary{
				{
					ID:   "lobby",
					Name: "Lobby",
					Settings: []protocol.DoorSettingSummary{
						{Name: "motd", Type: "textarea", Label: "Message of the day", Default: "Welcome", Value: "Welcome"},
					},
				},
				{ID: "chat", Name: "Chat"},
			}
			runtimeCtx.Admin = &protocol.RuntimeAdmin{
				Doors: runtimeCtx.Node.Doors,
				Users: []protocol.KnownUser{
					{PublicKey: "ed25519:test", Fingerprint: "test", Role: "admin", DisplayName: "Ada", Online: true},
				},
				Storage: protocol.RuntimeStorage{
					DatabasePath: "/tmp/phosphornet.db",
					StateRecords: []protocol.StateRecordSummary{
						{DoorID: "chat", Scope: "user", ScopeID: "test", Bytes: 42, UpdatedAt: "now"},
					},
				},
				Events: []protocol.RuntimeEvent{
					{Time: "now", Type: "door_open", DoorID: "chat", Fingerprint: "test", Message: "opened chat"},
				},
				Policy: map[string]any{},
			}

			response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
				ID:     "admin",
				Name:   "Station Admin",
				Entry:  "app.lua",
				Access: "admin",
			}, runtimeCtx)
			if err != nil {
				t.Fatalf("InvokeDoorView(%s) error = %v", page, err)
			}
			total, missing := panelGradientCounts(response.View)
			if total == 0 {
				t.Fatalf("admin page %s rendered no panels", page)
			}
			if missing != 0 {
				t.Fatalf("admin page %s rendered %d/%d panels without gradient backgrounds: %#v", page, missing, total, response.View)
			}
		})
	}
}

func TestInvokeAdminDoorAssignsRolesAndDoorRoleVisibility(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("admin")
	runtimeCtx.User.Role = "admin"

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "assign-role",
		Action: "assign_role",
		Values: map[string]string{
			"role-public-key": "ed25519:member",
			"role-name":       "moderator",
		},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(assign_role) error = %v", err)
	}
	if op, ok := adminOpFor(response.AdminOps, "set_user_role"); !ok || op.PublicKey != "ed25519:member" || op.Role != "moderator" {
		t.Fatalf("assign_role admin ops = %#v, want set_user_role", response.AdminOps)
	}

	runtimeCtx.State.Global = map[string]any{"roles": map[string]any{"ed25519:member": "moderator"}}
	response, err = InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "set-door-roles",
		Action: "set_door_roles",
		Values: map[string]string{
			"door-id":    "ops",
			"door-roles": "moderator, admin",
		},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(set_door_roles) error = %v", err)
	}
	if op, ok := adminOpFor(response.AdminOps, "set_door_roles"); !ok || op.DoorID != "ops" || len(op.Roles) != 2 {
		t.Fatalf("set_door_roles admin ops = %#v, want set_door_roles", response.AdminOps)
	}
}

func TestInvokeAdminDoorModerationOps(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("admin")
	runtimeCtx.User.Role = "admin"

	viewResponse, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "admin-moderation",
		Action: "nav:push:moderation",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(nav moderation) error = %v", err)
	}
	if !uiTreeContainsText(viewResponse.View, "Banned Keys") || !uiTreeContainsText(viewResponse.View, "Key Controls") {
		t.Fatalf("moderation view = %#v, want moderation panels", viewResponse.View)
	}

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "ban-key",
		Action: "ban_key",
		Values: map[string]string{
			"moderation-public-key": "ed25519:bad",
			"moderation-reason":     "spam",
		},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(ban_key) error = %v", err)
	}
	if op, ok := adminOpFor(response.AdminOps, "ban_key"); !ok || op.PublicKey != "ed25519:bad" || op.Reason != "spam" {
		t.Fatalf("ban_key admin ops = %#v, want ban_key with reason", response.AdminOps)
	}

	response, err = InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "set-user-rate-limit",
		Action: "set_user_rate_limit",
		Values: map[string]string{
			"moderation-public-key":        "ed25519:bad",
			"moderation-events-per-minute": "3",
			"moderation-opens-per-minute":  "1",
		},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(set_user_rate_limit) error = %v", err)
	}
	op, ok := adminOpFor(response.AdminOps, "set_user_rate_limit")
	if !ok || op.EventsPerMinute == nil || *op.EventsPerMinute != 3 || op.OpensPerMinute == nil || *op.OpensPerMinute != 1 {
		t.Fatalf("rate limit admin ops = %#v, want set_user_rate_limit with values", response.AdminOps)
	}
}

func TestInvokeAdminDoorTogglesDisabledDoors(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("admin")
	runtimeCtx.User.Role = "admin"
	runtimeCtx.Node.Doors = []protocol.DoorSummary{{ID: "chat", Name: "Chat"}}

	viewResponse, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "admin-doors",
		Action: "nav:push:doors",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(nav doors) error = %v", err)
	}
	if !uiTreeContainsComponent(viewResponse.View, "checkbox") {
		t.Fatalf("admin doors view = %#v, want hosted-door checkbox", viewResponse.View)
	}

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "door-enabled-chat",
		Action: "toggle_door_enabled",
		Values: map[string]string{"checked": "false"},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(toggle_door_enabled) error = %v", err)
	}
	if op, ok := adminOpFor(response.AdminOps, "set_door_enabled"); !ok || op.DoorID != "chat" || op.Enabled == nil || *op.Enabled {
		t.Fatalf("toggle_door_enabled admin ops = %#v, want set_door_enabled false", response.AdminOps)
	}

	runtimeCtx.State.Global = map[string]any{"disabled_doors": map[string]any{"chat": true}}
	runtimeCtx.Node.Doors = []protocol.DoorSummary{{ID: "chat", Name: "Chat", Disabled: true}}
	response, err = InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "door-enabled-chat",
		Action: "toggle_door_enabled",
		Values: map[string]string{"checked": "true"},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(toggle_door_enabled enable) error = %v", err)
	}
	op, ok := adminOpFor(response.AdminOps, "set_door_enabled")
	if !ok || op.Enabled == nil || !*op.Enabled {
		t.Fatalf("toggle_door_enabled enable admin ops = %#v, want set_door_enabled true", response.AdminOps)
	}
}

func TestInvokeAdminDoorReordersEnabledDoorsAndCapturesKeys(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("admin")
	runtimeCtx.User.Role = "admin"
	runtimeCtx.Node.Doors = []protocol.DoorSummary{
		{ID: "lobby", Name: "Lobby"},
		{ID: "chat", Name: "Chat"},
		{ID: "forum", Name: "Forum"},
	}

	viewResponse, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "admin-doors",
		Action: "nav:push:doors",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(nav doors) error = %v", err)
	}
	if !viewResponse.View.CaptureKeys {
		t.Fatalf("admin doors view = %#v, want capture_keys enabled", viewResponse.View)
	}
	if !uiTreeContainsComponent(viewResponse.View, "dynamic_list") {
		t.Fatalf("admin doors view = %#v, want dynamic_list in door ordering panel", viewResponse.View)
	}
	if uiTreeContainsText(viewResponse.View, "Move Earlier") || uiTreeContainsText(viewResponse.View, "Move Later") {
		t.Fatalf("admin doors view = %#v, want no duplicate per-door reorder cards", viewResponse.View)
	}

	runtimeCtx.State.User = map[string]any{"__nav_stack": []any{"doors"}}
	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindSelect,
		Target: "door-order-list",
		Action: "select_door_order:chat",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(select_door_order) error = %v", err)
	}
	if !stateOpsContainKey(response.StateOps, "selected_nav_door") {
		t.Fatalf("select_door_order state ops = %#v, want selected_nav_door update", response.StateOps)
	}

	runtimeCtx.State.User["selected_nav_door"] = "chat"
	response, err = InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind: protocol.EventKindKey,
		Key:  "=",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(key =) error = %v", err)
	}
	op, ok := adminOpFor(response.AdminOps, "reorder_doors")
	if !ok {
		t.Fatalf("key = admin ops = %#v, want reorder_doors", response.AdminOps)
	}
	if len(op.DoorOrder) != 3 || op.DoorOrder[0] != "chat" || op.DoorOrder[1] != "lobby" || op.DoorOrder[2] != "forum" {
		t.Fatalf("door_order value = %#v, want [chat lobby forum]", op.DoorOrder)
	}
}

func TestInvokeAdminDoorEditsDoorSettings(t *testing.T) {
	doorsRoot, err := filepath.Abs(filepath.Join("..", "..", "doors"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("admin")
	runtimeCtx.User.Role = "admin"
	runtimeCtx.State.User = map[string]any{"__nav_stack": []any{"settings/detail:lobby"}}
	runtimeCtx.Node.Doors = []protocol.DoorSummary{
		{
			ID:   "lobby",
			Name: "Lobby",
			Settings: []protocol.DoorSettingSummary{
				{Name: "motd", Type: "textarea", Label: "Message of the day", Default: "Welcome", Value: "Welcome"},
				{Name: "show_online_users", Type: "bool", Label: "Show online users", Default: true, Value: true},
			},
		},
	}

	viewResponse, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("InvokeDoorView(settings) error = %v", err)
	}
	if !uiTreeContainsText(viewResponse.View, "Message of the day") || !uiTreeContainsComponent(viewResponse.View, "textarea") {
		t.Fatalf("settings view = %#v, want setting editor", viewResponse.View)
	}

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "save-setting-lobby-motd",
		Action: "save_door_setting:lobby:motd",
		Values: map[string]string{"setting-lobby-motd": "Station night"},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(save_door_setting) error = %v", err)
	}
	op, ok := adminOpFor(response.AdminOps, "set_door_setting")
	if !ok || op.DoorID != "lobby" || op.SettingKey != "motd" || op.SettingValue != "Station night" {
		t.Fatalf("save setting admin ops = %#v, want set_door_setting motd", response.AdminOps)
	}

	response, err = InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:     "admin",
		Name:   "Station Admin",
		Entry:  "app.lua",
		Access: "admin",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "setting-lobby-show_online_users",
		Action: "save_door_setting_bool:lobby:show_online_users",
		Values: map[string]string{"checked": "false"},
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(save bool setting) error = %v", err)
	}
	op, ok = adminOpFor(response.AdminOps, "set_door_setting")
	if !ok || op.DoorID != "lobby" || op.SettingKey != "show_online_users" || op.SettingValue != false {
		t.Fatalf("save bool setting admin ops = %#v, want set_door_setting bool", response.AdminOps)
	}
}

func TestLuaSandboxDoesNotOpenOSByDefault(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoor(t, doorsRoot, "safe", `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({ ui.text(tostring(os)) })
end
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:    "safe",
		Name:  "Safe",
		Entry: "app.lua",
	}, testRuntimeContext("safe"))
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if got := response.View.Children[0].Text; got != "nil" {
		t.Fatalf("os global text = %q, want nil", got)
	}
}

func TestLuaSandboxIgnoresUnsafeProfileLibraries(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoor(t, doorsRoot, "unsafe", `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.text(tostring(io ~= nil)),
    ui.text(tostring(os ~= nil)),
    ui.text(tostring(debug ~= nil)),
    ui.text(tostring(package ~= nil))
  })
end
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:      "unsafe",
		Name:    "Unsafe",
		Entry:   "app.lua",
		Sandbox: LuaSandboxConfig{Profile: "unsafe"},
	}, testRuntimeContext("unsafe"))
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	for index, child := range response.View.Children {
		if got := child.Text; got != "false" {
			t.Fatalf("child %d text = %q, want false", index, got)
		}
	}
}

func TestStdioInvokerUsesCanonicalRuntimeRequest(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "echo", "app.py", `
import json
import sys

request = json.loads(sys.stdin.read())
print(json.dumps({
    "contract_version": request["contract_version"],
    "view": {
        "component": "screen",
        "children": [
            {"component": "text", "text": request["lifecycle"]},
            {"component": "text", "text": request["door"]["id"]},
        ],
    },
}))
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:        "echo",
		Name:      "Echo",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Isolation: hostIsolation(),
	}, testRuntimeContext("echo"))
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if !uiTreeContainsText(response.View, "view") || !uiTreeContainsText(response.View, "echo") {
		t.Fatalf("stdio view = %#v, want canonical request values", response.View)
	}
}

func TestStdioInvokerRejectsMalformedJSON(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "badjson", "app.py", `
import sys

print("not-json")
print("diagnostic line", file=sys.stderr)
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:        "badjson",
		Name:      "Bad JSON",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Isolation: hostIsolation(),
	}, testRuntimeContext("badjson"))
	if err == nil {
		t.Fatal("InvokeDoorView() error = nil, want malformed JSON error")
	}
	if !strings.Contains(err.Error(), "decode stdio door response") || !strings.Contains(err.Error(), "diagnostic line") {
		t.Fatalf("InvokeDoorView() error = %v, want decode error with stderr diagnostics", err)
	}
}

func TestStdioInvokerRejectsOversizedStdout(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "toobig", "app.py", `
print("x" * (300 * 1024))
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:        "toobig",
		Name:      "Too Big",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Isolation: hostIsolation(),
	}, testRuntimeContext("toobig"))
	if err == nil {
		t.Fatal("InvokeDoorView() error = nil, want oversized stdout error")
	}
	if !strings.Contains(err.Error(), "response exceeded limit") {
		t.Fatalf("InvokeDoorView() error = %v, want response limit error", err)
	}
	if got := protocol.ErrorCodeOf(err); got != protocol.ErrorRuntimeResourceLimit {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q", got, protocol.ErrorRuntimeResourceLimit)
	}
}

func TestStdioInvokerHonorsTimeout(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "slowpy", "app.py", `
import time
time.sleep(5)
`)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:        "slowpy",
		Name:      "Slow Python",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Isolation: hostIsolation(),
	}, testRuntimeContext("slowpy"))
	if err == nil {
		t.Fatal("InvokeDoorView() error = nil, want timeout")
	}
	if got := protocol.ErrorCodeOf(err); got != protocol.ErrorRuntimeTimeout {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q", got, protocol.ErrorRuntimeTimeout)
	}
}

func TestStdioInvokerRequiresPodmanImageUnlessHostOptOut(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "echo", "app.py", `
print('{"contract_version":"phosphornet.door.runtime.v1","view":{"component":"screen","children":[]}}')
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:      "echo",
		Name:    "Echo",
		Runtime: "stdio",
		Command: []string{"python3", "app.py"},
	}, testRuntimeContext("echo"))
	if err == nil {
		t.Fatal("InvokeDoorView() error = nil, want missing podman image error")
	}
	if !strings.Contains(err.Error(), "podman isolation requires image") {
		t.Fatalf("InvokeDoorView() error = %v, want missing podman image error", err)
	}
	if got := protocol.ErrorCodeOf(err); got != protocol.ErrorRuntimeImageMissing {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q", got, protocol.ErrorRuntimeImageMissing)
	}
}

func TestStdioInvokerDefaultsToPodmanIsolation(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "weather", "manifest.toml", "")
	installFakePodman(t, `
printf '{"contract_version":"phosphornet.door.runtime.v1","view":{"component":"screen","children":[]}}\n'
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:      "weather",
		Name:    "Weather",
		Runtime: "stdio",
		Isolation: DoorIsolationConfig{
			Image: "localhost/phosphornet/weather-door:0.1.0",
		},
	}, testRuntimeContext("weather"))
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if response.View.Component != "screen" {
		t.Fatalf("response.View.Component = %q, want screen", response.View.Component)
	}
}

func TestPodmanInvokerBuildsHardenedArgv(t *testing.T) {
	readOnly := true
	got, err := buildPodmanRunCommand(DoorManifest{
		ID:      "weather",
		Name:    "Weather",
		Runtime: "stdio",
		Command: []string{"/door/app"},
		Isolation: DoorIsolationConfig{
			Mode:      "podman",
			Image:     "localhost/phosphornet/weather-door:0.1.0",
			Network:   "none",
			ReadOnly:  &readOnly,
			Memory:    "128m",
			CPUs:      0.25,
			PidsLimit: 64,
		},
	})
	if err != nil {
		t.Fatalf("buildPodmanRunCommand() error = %v", err)
	}
	want := []string{
		"podman",
		"run",
		"--rm",
		"-i",
		"--network=none",
		"--memory=128m",
		"--cpus=0.25",
		"--pids-limit=64",
		"--security-opt=no-new-privileges",
		"--cap-drop=ALL",
		"--userns=keep-id",
		"--workdir=/door",
		"--read-only",
		"localhost/phosphornet/weather-door:0.1.0",
		"/door/app",
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("buildPodmanRunCommand() = %#v, want %#v", got, want)
	}
}

func TestPodmanInvokerRejectsMalformedJSON(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "weather", "manifest.toml", "")
	installFakePodman(t, `
printf 'not-json\n'
printf 'container diagnostic\n' >&2
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:      "weather",
		Name:    "Weather",
		Runtime: "stdio",
		Isolation: DoorIsolationConfig{
			Mode:  "podman",
			Image: "localhost/phosphornet/weather-door:0.1.0",
		},
	}, testRuntimeContext("weather"))
	if err == nil {
		t.Fatal("InvokeDoorView() error = nil, want malformed JSON error")
	}
	if !strings.Contains(err.Error(), "decode stdio door response") || !strings.Contains(err.Error(), "container diagnostic") {
		t.Fatalf("InvokeDoorView() error = %v, want decode error with stderr diagnostics", err)
	}
	if got := protocol.ErrorCodeOf(err); got != protocol.ErrorRuntimeBadOutput {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q", got, protocol.ErrorRuntimeBadOutput)
	}
}

func TestPodmanInvokerHonorsTimeout(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "slow", "manifest.toml", "")
	installFakePodman(t, `
sleep 5
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:      "slow",
		Name:    "Slow",
		Runtime: "stdio",
		Isolation: DoorIsolationConfig{
			Mode:      "podman",
			Image:     "localhost/phosphornet/slow-door:0.1.0",
			TimeoutMS: 100,
		},
	}, testRuntimeContext("slow"))
	if err == nil {
		t.Fatal("InvokeDoorView() error = nil, want timeout")
	}
	if got := protocol.ErrorCodeOf(err); got != protocol.ErrorRuntimeTimeout {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q", got, protocol.ErrorRuntimeTimeout)
	}
}

func TestPodmanInvokerCapsStderrDiagnostics(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "loud", "manifest.toml", "")
	installFakePodman(t, `
python3 -c 'import sys; sys.stderr.write("x" * (80 * 1024)); sys.exit(2)'
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:      "loud",
		Name:    "Loud",
		Runtime: "stdio",
		Isolation: DoorIsolationConfig{
			Mode:  "podman",
			Image: "localhost/phosphornet/loud-door:0.1.0",
		},
	}, testRuntimeContext("loud"))
	if err == nil {
		t.Fatal("InvokeDoorView() error = nil, want stderr cap error")
	}
	if !strings.Contains(err.Error(), "stderr exceeded limit") {
		t.Fatalf("InvokeDoorView() error = %v, want stderr cap error", err)
	}
	if got := protocol.ErrorCodeOf(err); got != protocol.ErrorRuntimeResourceLimit {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q", got, protocol.ErrorRuntimeResourceLimit)
	}
}

func TestLuaDoorStoreAndNavigationHelpers(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoor(t, doorsRoot, "forum", `
local ui = phosphornet.ui

function view(ctx)
  local posts = ctx.store:get("room", "posts", {})
  return ui.screen({
    ui.header(ctx.nav:current("home")),
    ui.text("posts: " .. tostring(#posts)),
    ui.nav_button("open-posts", "Posts", "posts"),
    ui.back_button("go-back"),
  })
end

function update(ctx, event)
  if ctx.nav:handle(event, "home") then
    return view(ctx)
  end
  if event and event.action == "add_post" then
    ctx.store:append("room", "posts", { title = "hello" }, 10)
    ctx.store:set("user", "profile", { name = "Ada" })
  end
  return view(ctx)
end
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("forum")
	viewResponse, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if len(viewResponse.StateOps) != 0 {
		t.Fatalf("InvokeDoorView() state ops = %#v, want none for nav reads", viewResponse.StateOps)
	}

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "compose",
		Action: "add_post",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(add_post) error = %v", err)
	}
	if op, ok := stateOpForKey(response.StateOps, "posts"); !ok || op.Scope != protocol.StateScopeRoom {
		t.Fatalf("store append ops = %#v, want room posts op", response.StateOps)
	}
	if op, ok := stateOpForKey(response.StateOps, "profile"); !ok || op.Scope != protocol.StateScopeUser {
		t.Fatalf("store set ops = %#v, want user profile op", response.StateOps)
	}

	response, err = InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "open-posts",
		Action: "nav:push:posts",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(nav push) error = %v", err)
	}
	op, ok := stateOpForKey(response.StateOps, "__nav_stack")
	if !ok {
		t.Fatalf("nav push ops = %#v, want __nav_stack op", response.StateOps)
	}
	stack, ok := op.Value.([]any)
	if !ok || len(stack) != 1 || stack[0] != "posts" {
		t.Fatalf("nav push stack = %#v, want [posts]", op.Value)
	}

	runtimeCtx.State.User = map[string]any{"__nav_stack": []any{"home", "posts"}}
	response, err = InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:    "forum",
		Name:  "Forum",
		Entry: "app.lua",
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "go-back",
		Action: "nav:back",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(nav back) error = %v", err)
	}
	op, ok = stateOpForKey(response.StateOps, "__nav_stack")
	if !ok {
		t.Fatalf("nav back ops = %#v, want __nav_stack op", response.StateOps)
	}
	stack, ok = op.Value.([]any)
	if !ok || len(stack) != 1 || stack[0] != "home" {
		t.Fatalf("nav back stack = %#v, want [home]", op.Value)
	}
}

func TestLuaDoorReceivesResolvedSettings(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoor(t, doorsRoot, "settings", `
local ui = phosphornet.ui

function view(ctx)
  return ui.screen({
    ui.text(ctx.settings.motd),
    ui.text(tostring(ctx.settings.show_online_users)),
    ui.text(tostring(ctx.settings.board_size))
  })
end
`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("settings")
	runtimeCtx.Settings = map[string]any{
		"motd":              "Station night",
		"show_online_users": false,
		"board_size":        9,
	}
	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:    "settings",
		Name:  "Settings",
		Entry: "app.lua",
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if !uiTreeContainsText(response.View, "Station night") || !uiTreeContainsText(response.View, "false") || !uiTreeContainsText(response.View, "9") {
		t.Fatalf("settings view = %#v, want ctx.settings values", response.View)
	}
}

func TestPythonDoorStoreAndNavigationHelpers(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "pyforum", "app.py", `
import asyncio

from phosphornet import run_module, ui


async def view(ctx):
    posts = ctx.store.get("room", "posts", [])
    return ui.screen(
        [
            ui.header(ctx.nav.current("home")),
            ui.text(f"posts: {len(posts)}"),
            ui.nav_button("open-posts", "Posts", "posts"),
            ui.back_button("go-back"),
        ]
    )


async def update(ctx, event):
    if ctx.nav.handle(event, default="home"):
        return await view(ctx)
    if event.get("action") == "add_post":
        ctx.store.append("room", "posts", {"title": "hello"}, limit=10)
        ctx.store.set("user", "profile", {"name": "Ada"})
    return await view(ctx)


if __name__ == "__main__":
    asyncio.run(run_module(globals()))
`)
	includePythonSDK(t, doorsRoot, "pyforum")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("pyforum")
	viewResponse, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:        "pyforum",
		Name:      "Python Forum",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Isolation: hostIsolation(),
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if len(viewResponse.StateOps) != 0 {
		t.Fatalf("InvokeDoorView() state ops = %#v, want none for nav reads", viewResponse.StateOps)
	}

	response, err := InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:        "pyforum",
		Name:      "Python Forum",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Isolation: hostIsolation(),
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "compose",
		Action: "add_post",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(add_post) error = %v", err)
	}
	if op, ok := stateOpForKey(response.StateOps, "posts"); !ok || op.Scope != protocol.StateScopeRoom {
		t.Fatalf("store append ops = %#v, want room posts op", response.StateOps)
	}
	if op, ok := stateOpForKey(response.StateOps, "profile"); !ok || op.Scope != protocol.StateScopeUser {
		t.Fatalf("store set ops = %#v, want user profile op", response.StateOps)
	}

	response, err = InvokeDoorUpdate(ctx, doorsRoot, DoorManifest{
		ID:        "pyforum",
		Name:      "Python Forum",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Isolation: hostIsolation(),
	}, runtimeCtx, protocol.UIEvent{
		Kind:   protocol.EventKindAction,
		Target: "open-posts",
		Action: "nav:push:posts",
	})
	if err != nil {
		t.Fatalf("InvokeDoorUpdate(nav push) error = %v", err)
	}
	op, ok := stateOpForKey(response.StateOps, "__nav_stack")
	if !ok {
		t.Fatalf("nav push ops = %#v, want __nav_stack op", response.StateOps)
	}
	stack, ok := op.Value.([]any)
	if !ok || len(stack) != 1 || stack[0] != "posts" {
		t.Fatalf("nav push stack = %#v, want [posts]", op.Value)
	}
}

func TestPythonDoorReceivesResolvedSettings(t *testing.T) {
	doorsRoot := t.TempDir()
	writeTestDoorFile(t, doorsRoot, "pysettings", "app.py", `
import asyncio

from phosphornet import run_module, ui


async def view(ctx):
    return ui.screen(
        [
            ui.text(ctx.settings["motd"]),
            ui.text(str(ctx.settings["show_online_users"])),
            ui.text(str(ctx.settings["board_size"])),
        ]
    )


if __name__ == "__main__":
    asyncio.run(run_module(globals()))
`)
	includePythonSDK(t, doorsRoot, "pysettings")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeCtx := testRuntimeContext("pysettings")
	runtimeCtx.Settings = map[string]any{
		"motd":              "Station night",
		"show_online_users": false,
		"board_size":        9,
	}
	response, err := InvokeDoorView(ctx, doorsRoot, DoorManifest{
		ID:        "pysettings",
		Name:      "Python Settings",
		Runtime:   "stdio",
		Command:   []string{"python3", "app.py"},
		Isolation: hostIsolation(),
	}, runtimeCtx)
	if err != nil {
		t.Fatalf("InvokeDoorView() error = %v", err)
	}
	if !uiTreeContainsText(response.View, "Station night") || !uiTreeContainsText(response.View, "False") || !uiTreeContainsText(response.View, "9") {
		t.Fatalf("settings view = %#v, want ctx.settings values", response.View)
	}
}

func stateOpsContainKey(ops []protocol.StateOp, key string) bool {
	for _, op := range ops {
		if op.Key == key {
			return true
		}
	}
	return false
}

func stateOpForKey(ops []protocol.StateOp, key string) (protocol.StateOp, bool) {
	for _, op := range ops {
		if op.Key == key {
			return op, true
		}
	}
	return protocol.StateOp{}, false
}

func adminOpFor(ops []protocol.AdminOp, opName string) (protocol.AdminOp, bool) {
	for _, op := range ops {
		if op.Op == opName {
			return op, true
		}
	}
	return protocol.AdminOp{}, false
}

func uiTreeContainsComponent(node protocol.UINode, component string) bool {
	if node.Component == component {
		return true
	}
	for _, child := range node.Children {
		if uiTreeContainsComponent(child, component) {
			return true
		}
	}
	return false
}

func panelGradientCounts(node protocol.UINode) (total int, missing int) {
	if node.Component == "panel" {
		total++
		if node.Style == nil || node.Style.Background == nil || node.Style.Background.Kind != "gradient" {
			missing++
		}
	}
	for _, child := range node.Children {
		childTotal, childMissing := panelGradientCounts(child)
		total += childTotal
		missing += childMissing
	}
	return total, missing
}

func uiTreeContainsText(node protocol.UINode, text string) bool {
	if strings.Contains(node.Text, text) || strings.Contains(node.Title, text) || strings.Contains(node.Value, text) || strings.Contains(node.Placeholder, text) {
		return true
	}
	for _, item := range node.Items {
		if strings.Contains(item.Label, text) {
			return true
		}
	}
	for _, row := range node.Rows {
		for _, cell := range row {
			if strings.Contains(cell, text) {
				return true
			}
		}
	}
	for _, child := range node.Children {
		if uiTreeContainsText(child, text) {
			return true
		}
	}
	return false
}

func testRuntimeContext(doorID string) protocol.RuntimeContext {
	return protocol.RuntimeContext{
		Session: protocol.RuntimeSession{ID: "test-session"},
		User: protocol.RuntimeUser{
			PublicKey:   "ed25519:test",
			Fingerprint: "test",
			Role:        "member",
		},
		Node: protocol.RuntimeNode{
			ID:   "node:test",
			Name: "Test Node",
		},
		Room: protocol.RuntimeRoom{
			ID:     "door:" + doorID,
			DoorID: doorID,
		},
		State: protocol.RuntimeStateSnapshot{
			User:   map[string]any{},
			Room:   map[string]any{},
			Global: map[string]any{},
		},
	}
}

func writeTestDoor(t *testing.T, doorsRoot string, doorID string, source string) {
	t.Helper()
	writeTestDoorFile(t, doorsRoot, doorID, "app.lua", source)
}

func writeTestDoorFile(t *testing.T, doorsRoot string, doorID string, filename string, source string) {
	t.Helper()
	doorDir := filepath.Join(doorsRoot, doorID)
	if err := os.MkdirAll(doorDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(doorDir, filename), []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func includePythonSDK(t *testing.T, doorsRoot string, doorID string) {
	t.Helper()
	sdkPath, err := filepath.Abs(filepath.Join("..", "..", "sdk", "python", "phosphornet"))
	if err != nil {
		t.Fatalf("Abs(sdk python) error = %v", err)
	}
	doorDir := filepath.Join(doorsRoot, doorID)
	if err := os.Symlink(sdkPath, filepath.Join(doorDir, "phosphornet")); err != nil {
		t.Fatalf("Symlink(python sdk) error = %v", err)
	}
}

func installFakePodman(t *testing.T, body string) {
	t.Helper()
	binDir := t.TempDir()
	path := filepath.Join(binDir, "podman")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(fake podman) error = %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func hostIsolation() DoorIsolationConfig {
	return DoorIsolationConfig{Mode: IsolationModeHost}
}
