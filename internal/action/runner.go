package action

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Runner struct {
	Config Config
}

func (r Runner) Run(ctx context.Context, request Request) Response {
	response := Response{
		ProtocolVersion: ProtocolVersion,
		RequestID:       request.RequestID,
		RuleID:          request.RuleID,
		ExitCode:        -1,
	}
	if request.ProtocolVersion != ProtocolVersion {
		response.Error = fmt.Sprintf("unsupported action protocol version %q", request.ProtocolVersion)
		return response
	}
	if request.RequestID == "" {
		response.Error = "request_id is required"
		return response
	}
	if len(request.RequestID) > 128 {
		response.Error = "request_id exceeds 128 bytes"
		return response
	}
	if !RuleIDPattern.MatchString(request.RuleID) {
		response.Error = fmt.Sprintf("invalid action rule id %q", request.RuleID)
		return response
	}
	if request.DoorID == "" {
		response.Error = "door_id is required"
		return response
	}
	rule, ok := r.Config.Rule(request.RuleID)
	if !ok {
		response.Error = fmt.Sprintf("unknown action rule %q", request.RuleID)
		return response
	}
	if !rule.AllowsDoor(request.DoorID) {
		response.Error = fmt.Sprintf("door %q is not allowed to run action %q", request.DoorID, request.RuleID)
		return response
	}

	timeoutMS := rule.TimeoutMS
	if timeoutMS == 0 {
		timeoutMS = DefaultTimeoutMS
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	input, err := json.Marshal(request.Input)
	if err != nil {
		response.Error = fmt.Sprintf("encode action input: %v", err)
		return response
	}
	input = append(input, '\n')
	cmd := newCommandContext(runCtx, rule.Command[0], rule.Command[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Dir = rule.WorkingDir
	cmd.Env = rule.EnvironmentList()
	stdout := &limitedBuffer{limit: r.Config.MaxOutputBytes}
	stderr := &limitedBuffer{limit: r.Config.MaxOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	response.Stdout = stdout.String()
	response.Stderr = stderr.String()
	response.Truncated = stdout.truncated || stderr.truncated
	if cmd.ProcessState != nil {
		response.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runCtx.Err() != nil {
		response.TimedOut = errors.Is(runCtx.Err(), context.DeadlineExceeded)
		response.Error = runCtx.Err().Error()
		return response
	}
	if err != nil {
		response.Error = err.Error()
		return response
	}
	response.OK = true
	return response
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || originalLen > 0
		return originalLen, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buf.Write(p)
	return originalLen, nil
}

func (b *limitedBuffer) String() string { return b.buf.String() }
