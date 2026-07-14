package protocol

const RuntimeContractVersion = "phosphornet.door.runtime.v1"

type Lifecycle string

const (
	LifecycleInit    Lifecycle = "init"
	LifecycleView    Lifecycle = "view"
	LifecycleUpdate  Lifecycle = "update"
	LifecycleOnJoin  Lifecycle = "on_join"
	LifecycleOnLeave Lifecycle = "on_leave"
	LifecycleTick    Lifecycle = "tick"
)

type EventKind string

const (
	EventKindAction EventKind = "action"
	EventKindSelect EventKind = "select"
	EventKindSubmit EventKind = "submit"
	EventKindKey    EventKind = "key"
	EventKindFocus  EventKind = "focus"
	// EventKindActionResult is node-generated and is never accepted from a client.
	EventKindActionResult EventKind = "action_result"
)

type StateScope string

const (
	StateScopeUser   StateScope = "user"
	StateScopeRoom   StateScope = "room"
	StateScopeGlobal StateScope = "global"
)

type StateOpKind string

const (
	StateOpSet     StateOpKind = "set"
	StateOpDelete  StateOpKind = "delete"
	StateOpClear   StateOpKind = "clear"
	StateOpReplace StateOpKind = "replace"
)

type BroadcastScope string

const (
	BroadcastScopeRoom BroadcastScope = "room"
	BroadcastScopeDoor BroadcastScope = "door"
	BroadcastScopeUser BroadcastScope = "user"
)

type NotifyTarget string

const (
	NotifyTargetSelf NotifyTarget = "self"
	NotifyTargetRoom NotifyTarget = "room"
	NotifyTargetDoor NotifyTarget = "door"
	NotifyTargetUser NotifyTarget = "user"
	NotifyTargetAll  NotifyTarget = "all"
)

type TransitionKind string

const (
	TransitionKindOpenDoor  TransitionKind = "open_door"
	TransitionKindCloseDoor TransitionKind = "close_door"
	TransitionKindRoom      TransitionKind = "room"
)

type RuntimeDoor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RuntimeUser struct {
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name,omitempty"`
	Bio         string `json:"bio,omitempty"`
	StatusLine  string `json:"status_line,omitempty"`
	Guest       bool   `json:"guest,omitempty"`
}

type RuntimeNode struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Fingerprint      string        `json:"fingerprint,omitempty"`
	AccessMode       string        `json:"access_mode,omitempty"`
	MaintenanceMode  bool          `json:"maintenance_mode,omitempty"`
	StationAllowlist []string      `json:"station_allowlist,omitempty"`
	Admins           []string      `json:"admins,omitempty"`
	DoorsDir         string        `json:"doors_dir,omitempty"`
	DatabasePath     string        `json:"database_path,omitempty"`
	DefaultRuntime   string        `json:"default_runtime,omitempty"`
	LuaSandbox       RuntimeLua    `json:"lua_sandbox,omitempty"`
	Doors            []DoorSummary `json:"doors,omitempty"`
}

type RuntimeRoom struct {
	ID     string `json:"id"`
	DoorID string `json:"door_id"`
}

type RuntimeSession struct {
	ID string `json:"id"`
}

type RuntimePermissions struct {
	Role           string   `json:"role"`
	Capabilities   []string `json:"capabilities,omitempty"`
	CanWriteGlobal bool     `json:"can_write_global"`
	Muted          bool     `json:"muted,omitempty"`
}

type RuntimeLua struct {
	Profile        string   `json:"profile,omitempty"`
	Libraries      []string `json:"libraries,omitempty"`
	MaxMemoryKB    int      `json:"max_memory_kb,omitempty"`
	MaxExecutionMS int      `json:"max_execution_ms,omitempty"`
}

type RuntimeStateSnapshot struct {
	User   map[string]any `json:"user"`
	Room   map[string]any `json:"room"`
	Global map[string]any `json:"global"`
}

type PresenceUser struct {
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name,omitempty"`
	StatusLine  string `json:"status_line,omitempty"`
	ActiveDoor  string `json:"active_door,omitempty"`
	Guest       bool   `json:"guest,omitempty"`
}

type PresenceSnapshot struct {
	RoomUsers []PresenceUser `json:"room_users"`
	DoorUsers []PresenceUser `json:"door_users"`
	AllUsers  []PresenceUser `json:"all_users,omitempty"`
}

type KnownUser struct {
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name,omitempty"`
	Bio         string `json:"bio,omitempty"`
	StatusLine  string `json:"status_line,omitempty"`
	Guest       bool   `json:"guest,omitempty"`
	FirstSeen   string `json:"first_seen,omitempty"`
	LastSeen    string `json:"last_seen,omitempty"`
	Online      bool   `json:"online,omitempty"`
}

type StateRecordSummary struct {
	DoorID    string `json:"door_id"`
	Scope     string `json:"scope"`
	ScopeID   string `json:"scope_id"`
	Bytes     int    `json:"bytes"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type RuntimeStorage struct {
	DatabasePath string               `json:"database_path,omitempty"`
	StateRecords []StateRecordSummary `json:"state_records,omitempty"`
}

type RuntimeEvent struct {
	Time        string `json:"time"`
	Type        string `json:"type"`
	DoorID      string `json:"door_id,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Message     string `json:"message"`
}

type RuntimeContext struct {
	Session     RuntimeSession       `json:"session"`
	User        RuntimeUser          `json:"user"`
	Node        RuntimeNode          `json:"node"`
	Room        RuntimeRoom          `json:"room"`
	State       RuntimeStateSnapshot `json:"state"`
	Settings    map[string]any       `json:"settings,omitempty"`
	Presence    PresenceSnapshot     `json:"presence"`
	Users       []KnownUser          `json:"users,omitempty"`
	Storage     RuntimeStorage       `json:"storage,omitempty"`
	Events      []RuntimeEvent       `json:"events,omitempty"`
	Admin       *RuntimeAdmin        `json:"admin,omitempty"`
	Permissions RuntimePermissions   `json:"permissions"`
}

type RuntimeAdmin struct {
	StationAllowlist []string       `json:"station_allowlist,omitempty"`
	Admins           []string       `json:"admins,omitempty"`
	Users            []KnownUser    `json:"users,omitempty"`
	Doors            []DoorSummary  `json:"doors,omitempty"`
	Storage          RuntimeStorage `json:"storage,omitempty"`
	Events           []RuntimeEvent `json:"events,omitempty"`
	Policy           map[string]any `json:"policy,omitempty"`
	DatabasePath     string         `json:"database_path,omitempty"`
	DoorsDir         string         `json:"doors_dir,omitempty"`
	DefaultRuntime   string         `json:"default_runtime,omitempty"`
	LuaSandbox       RuntimeLua     `json:"lua_sandbox,omitempty"`
}

type RuntimeRequest struct {
	ContractVersion string         `json:"contract_version"`
	Door            RuntimeDoor    `json:"door"`
	Lifecycle       Lifecycle      `json:"lifecycle"`
	Context         RuntimeContext `json:"ctx"`
	Event           *UIEvent       `json:"event,omitempty"`
}

type StateOp struct {
	Scope StateScope  `json:"scope"`
	Op    StateOpKind `json:"op"`
	Key   string      `json:"key,omitempty"`
	Value any         `json:"value,omitempty"`
}

type BroadcastEffect struct {
	Scope         BroadcastScope `json:"scope"`
	DoorID        string         `json:"door_id,omitempty"`
	RoomID        string         `json:"room_id,omitempty"`
	UserPublicKey string         `json:"user_public_key,omitempty"`
	Event         UIEvent        `json:"event"`
}

type NotifyEffect struct {
	Target        NotifyTarget `json:"target"`
	Level         string       `json:"level"`
	Message       string       `json:"message"`
	UserPublicKey string       `json:"user_public_key,omitempty"`
}

type TransitionEffect struct {
	Kind   TransitionKind `json:"kind"`
	DoorID string         `json:"door_id,omitempty"`
	RoomID string         `json:"room_id,omitempty"`
}

type ActionEffect struct {
	RequestID string `json:"request_id"`
	RuleID    string `json:"rule_id"`
	Input     any    `json:"input,omitempty"`
}

type ActionResult struct {
	RequestID string `json:"request_id"`
	RuleID    string `json:"rule_id"`
	OK        bool   `json:"ok"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Error     string `json:"error,omitempty"`
	TimedOut  bool   `json:"timed_out,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type ProfileUpdateEffect struct {
	DisplayName *string `json:"display_name,omitempty"`
	Bio         *string `json:"bio,omitempty"`
	StatusLine  *string `json:"status_line,omitempty"`
	Reset       bool    `json:"reset,omitempty"`
}

type AdminOp struct {
	Op              string   `json:"op"`
	PublicKey       string   `json:"public_key,omitempty"`
	Role            string   `json:"role,omitempty"`
	DoorID          string   `json:"door_id,omitempty"`
	Enabled         *bool    `json:"enabled,omitempty"`
	Roles           []string `json:"roles,omitempty"`
	DoorOrder       []string `json:"door_order,omitempty"`
	SettingKey      string   `json:"setting_key,omitempty"`
	SettingValue    any      `json:"setting_value,omitempty"`
	Reset           bool     `json:"reset,omitempty"`
	Message         string   `json:"message,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	ExpiresAt       string   `json:"expires_at,omitempty"`
	EventsPerMinute *int     `json:"events_per_minute,omitempty"`
	OpensPerMinute  *int     `json:"opens_per_minute,omitempty"`
	Level           string   `json:"level,omitempty"`
	Maintenance     *bool    `json:"maintenance,omitempty"`
	NotifyTargets   bool     `json:"notify_targets,omitempty"`
}

type RuntimeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type RuntimeResponse struct {
	ContractVersion string                `json:"contract_version"`
	View            UINode                `json:"view"`
	StateOps        []StateOp             `json:"state_ops,omitempty"`
	Broadcasts      []BroadcastEffect     `json:"broadcasts,omitempty"`
	Notifies        []NotifyEffect        `json:"notifies,omitempty"`
	Transitions     []TransitionEffect    `json:"transitions,omitempty"`
	Actions         []ActionEffect        `json:"actions,omitempty"`
	ProfileUpdates  []ProfileUpdateEffect `json:"profile_updates,omitempty"`
	AdminOps        []AdminOp             `json:"admin_ops,omitempty"`
	Error           *RuntimeError         `json:"error,omitempty"`
}
