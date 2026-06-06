package protocol

import "phosphornet/internal/identity"

const (
	TypeHello      = "hello"
	TypeChallenge  = "challenge"
	TypeAuth       = "auth"
	TypeAuthOK     = "auth_ok"
	TypeAuthDenied = "auth_denied"
	TypeDoorList   = "door_list"
	TypeOpenDoor   = "open_door"
	TypeRender     = "render"
	TypeNotify     = "notify"
	TypeError      = "error"
	TypeEvent      = "event"
)

type HelloMessage struct {
	Type                   string       `json:"type"`
	ClientPublicKey        string       `json:"client_public_key"`
	ClientVersion          string       `json:"client_version"`
	RuntimeProtocolVersion string       `json:"runtime_protocol_version"`
	JSONUISchemaVersion    string       `json:"json_ui_schema_version"`
	SupportedComponents    []string     `json:"supported_components"`
	SupportedStyleFeatures []string     `json:"supported_style_features"`
	SupportedEventKinds    []EventKind  `json:"supported_event_kinds"`
	RenderLimits           RenderLimits `json:"render_limits"`
}

type ChallengeMessage struct {
	Type      string                        `json:"type"`
	Payload   identity.NodeChallengePayload `json:"payload"`
	Signature string                        `json:"signature"`
}

type AuthMessage struct {
	Type      string                `json:"type"`
	Payload   identity.LoginPayload `json:"payload"`
	Signature string                `json:"signature"`
}

type AuthOKMessage struct {
	Type        string `json:"type"`
	NodeID      string `json:"node_id"`
	NodeName    string `json:"node_name"`
	Role        string `json:"role"`
	Fingerprint string `json:"fingerprint"`
}

type AuthDeniedMessage struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type DoorSummary struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Runtime        string               `json:"runtime,omitempty"`
	Visibility     string               `json:"visibility,omitempty"`
	Access         string               `json:"access,omitempty"`
	AllowlistCount int                  `json:"allowlist_count,omitempty"`
	Entry          string               `json:"entry,omitempty"`
	Command        []string             `json:"command,omitempty"`
	SandboxProfile string               `json:"sandbox_profile,omitempty"`
	Roles          []string             `json:"roles,omitempty"`
	Disabled       bool                 `json:"disabled,omitempty"`
	Settings       []DoorSettingSummary `json:"settings,omitempty"`
}

type DoorSettingSummary struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Label   string   `json:"label,omitempty"`
	Default any      `json:"default,omitempty"`
	Value   any      `json:"value,omitempty"`
	Options []string `json:"options,omitempty"`
}

type DoorListMessage struct {
	Type  string        `json:"type"`
	Doors []DoorSummary `json:"doors"`
}

type OpenDoorMessage struct {
	Type   string `json:"type"`
	DoorID string `json:"door_id"`
}

type RenderMessage struct {
	Type           string `json:"type"`
	SessionID      string `json:"session_id"`
	ActiveDoorID   string `json:"active_door_id"`
	RenderRevision int64  `json:"render_revision"`
	View           UINode `json:"view"`
}

type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type RenderLimits struct {
	MaxWebSocketMessageBytes   int `json:"max_websocket_message_bytes"`
	MaxUINodeDepth             int `json:"max_ui_node_depth"`
	MaxUIChildren              int `json:"max_ui_children"`
	MaxUIItems                 int `json:"max_ui_items"`
	MaxUIGradientStops         int `json:"max_ui_gradient_stops"`
	MaxGridRows                int `json:"max_grid_rows"`
	MaxGridCols                int `json:"max_grid_cols"`
	MaxSingleLineTextRunes     int `json:"max_single_line_text_runes"`
	MaxMultilineTextRunes      int `json:"max_multiline_text_runes"`
	MaxChromeTextRunes         int `json:"max_chrome_text_runes"`
	MaxRenderMessagesPerSecond int `json:"max_render_messages_per_second"`
	MaxNotificationsPerMinute  int `json:"max_notifications_per_minute"`
}

type NotifyMessage struct {
	Type    string `json:"type"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type EventMessage struct {
	Type           string  `json:"type"`
	SessionID      string  `json:"session_id"`
	ActiveDoorID   string  `json:"active_door_id"`
	RenderRevision int64   `json:"render_revision"`
	EventID        string  `json:"event_id"`
	Event          UIEvent `json:"event"`
}

type UIEvent struct {
	Kind   EventKind         `json:"kind"`
	Target string            `json:"target,omitempty"`
	Action string            `json:"action,omitempty"`
	Key    string            `json:"key,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

type UINode struct {
	Component   string     `json:"component"`
	ID          string     `json:"id,omitempty"`
	Text        string     `json:"text,omitempty"`
	Title       string     `json:"title,omitempty"`
	Style       *UIStyle   `json:"style,omitempty"`
	Action      string     `json:"action,omitempty"`
	Scroll      string     `json:"scroll,omitempty"`
	CaptureKeys bool       `json:"capture_keys,omitempty"`
	Dock        string     `json:"dock,omitempty"`
	Value       string     `json:"value,omitempty"`
	Checked     bool       `json:"checked,omitempty"`
	Placeholder string     `json:"placeholder,omitempty"`
	Items       []Item     `json:"items,omitempty"`
	Rows        [][]string `json:"rows,omitempty"`
	Children    []UINode   `json:"children,omitempty"`
}

type UIStyle struct {
	Background *UIBackground `json:"background,omitempty"`
}

type UIBackground struct {
	Kind      string           `json:"kind,omitempty"`
	Direction string           `json:"direction,omitempty"`
	Color     string           `json:"color,omitempty"`
	From      string           `json:"from,omitempty"`
	To        string           `json:"to,omitempty"`
	Stops     []UIGradientStop `json:"stops,omitempty"`
}

type UIGradientStop struct {
	At    float64 `json:"at"`
	Color string  `json:"color"`
}

type Item struct {
	Label  string `json:"label"`
	Action string `json:"action,omitempty"`
}

func Screen(children ...UINode) UINode {
	return UINode{Component: "screen", Children: children}
}

func Header(text string) UINode {
	return UINode{Component: "header", Text: text}
}

func Status(text string) UINode {
	return UINode{Component: "status", Text: text}
}

func Text(text string) UINode {
	return UINode{Component: "text", Text: text}
}

func Markdown(text string) UINode {
	return UINode{Component: "markdown", Text: text}
}

func Panel(title string, children ...UINode) UINode {
	return UINode{Component: "panel", Title: title, Children: children}
}

func Menu(id string, items ...Item) UINode {
	return UINode{Component: "menu", ID: id, Items: items}
}

func List(id string, items ...Item) UINode {
	return UINode{Component: "list", ID: id, Items: items}
}

func DynamicList(id string, items ...Item) UINode {
	return UINode{Component: "dynamic_list", ID: id, Items: items}
}

func Button(id, text, action string) UINode {
	return UINode{Component: "button", ID: id, Text: text, Action: action}
}

func Checkbox(id, text string, checked bool, action string) UINode {
	return UINode{Component: "checkbox", ID: id, Text: text, Checked: checked, Action: action}
}

func Input(id, placeholder, value string) UINode {
	return UINode{Component: "input", ID: id, Placeholder: placeholder, Value: value}
}

func Textarea(id, placeholder, value string) UINode {
	return UINode{Component: "textarea", ID: id, Placeholder: placeholder, Value: value}
}

func Grid(id string, rows [][]string) UINode {
	return UINode{Component: "grid", ID: id, Rows: rows}
}
