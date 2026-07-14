package action

const ProtocolVersion = "phosphornet.action.v1"

type Request struct {
	ProtocolVersion string `json:"protocol_version"`
	RequestID       string `json:"request_id"`
	RuleID          string `json:"rule_id"`
	DoorID          string `json:"door_id"`
	User            User   `json:"user"`
	Input           any    `json:"input,omitempty"`
}

type User struct {
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	Role        string `json:"role"`
}

type Response struct {
	ProtocolVersion string `json:"protocol_version"`
	RequestID       string `json:"request_id"`
	RuleID          string `json:"rule_id"`
	OK              bool   `json:"ok"`
	ExitCode        int    `json:"exit_code"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	Error           string `json:"error,omitempty"`
	TimedOut        bool   `json:"timed_out,omitempty"`
	Truncated       bool   `json:"truncated,omitempty"`
}
