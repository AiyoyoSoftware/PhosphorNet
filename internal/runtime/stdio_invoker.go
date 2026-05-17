package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"phosphornet/internal/protocol"
)

const (
	maxStdioStdoutBytes = 256 * 1024
	maxStdioStderrBytes = 64 * 1024
)

type StdioInvoker struct{}

func (i StdioInvoker) Invoke(ctx context.Context, doorsRoot string, manifest DoorManifest, _ RuntimeOptions, request protocol.RuntimeRequest) (protocol.RuntimeResponse, error) {
	mode := strings.ToLower(strings.TrimSpace(manifest.Isolation.Mode))
	if mode == "" {
		mode = IsolationModePodman
	}
	if mode == IsolationModePodman {
		manifest.Isolation.Mode = IsolationModePodman
		return ContainerInvoker{}.Invoke(ctx, doorsRoot, manifest, RuntimeOptions{}, request)
	}
	if mode != IsolationModeHost {
		message := fmt.Sprintf("stdio runtime for door %q requires isolation mode %q or %q", manifest.ID, IsolationModeHost, IsolationModePodman)
		return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeDeniedPolicy, message, nil)
	}

	doorDir, err := resolveDoorDir(doorsRoot, manifest)
	if err != nil {
		return protocol.RuntimeResponse{}, err
	}

	command := append([]string{}, manifest.Command...)
	if len(command) == 0 {
		message := fmt.Sprintf("stdio runtime for door %q requires command", manifest.ID)
		return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeNotAvailable, message, nil)
	}

	return invokeStdioProcess(ctx, command, doorDir, os.Environ(), stdioTimeout(manifest), request)
}

func resolveDoorEntryPath(doorsRoot string, manifest DoorManifest) (string, error) {
	if manifest.Entry == "" {
		return "", fmt.Errorf("door %q has no entry", manifest.ID)
	}
	doorDirReal, doorDirAbs, err := resolveDoorDirPaths(doorsRoot, manifest)
	if err != nil {
		return "", err
	}
	entryPath, err := filepath.Abs(filepath.Join(doorDirAbs, manifest.Entry))
	if err != nil {
		return "", fmt.Errorf("resolve door entry: %w", err)
	}
	entryInfo, err := os.Stat(entryPath)
	if err != nil {
		return "", fmt.Errorf("stat door entry: %w", err)
	}
	if entryInfo.IsDir() {
		return "", fmt.Errorf("door %q entry %q is a directory", manifest.ID, manifest.Entry)
	}
	entryReal, err := filepath.EvalSymlinks(entryPath)
	if err != nil {
		return "", fmt.Errorf("resolve door entry symlinks: %w", err)
	}
	if !pathWithinRoot(doorDirReal, entryReal) {
		return "", fmt.Errorf("door %q entry escapes door directory", manifest.ID)
	}
	return entryReal, nil
}

func resolveDoorDir(doorsRoot string, manifest DoorManifest) (string, error) {
	_, doorDirAbs, err := resolveDoorDirPaths(doorsRoot, manifest)
	return doorDirAbs, err
}

func resolveDoorDirPaths(doorsRoot string, manifest DoorManifest) (string, string, error) {
	rootAbs, err := filepath.Abs(doorsRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve doors root: %w", err)
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", "", fmt.Errorf("resolve doors root symlinks: %w", err)
	}
	doorDir := manifest.Dir
	if strings.TrimSpace(doorDir) == "" {
		doorDir = filepath.Join(rootAbs, manifest.ID)
	}
	doorDirAbs, err := filepath.Abs(doorDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve door dir: %w", err)
	}
	doorDirReal, err := filepath.EvalSymlinks(doorDirAbs)
	if err != nil {
		return "", "", fmt.Errorf("resolve door dir symlinks: %w", err)
	}
	if !pathWithinRoot(rootReal, doorDirReal) {
		return "", "", fmt.Errorf("door %q directory escapes doors root", manifest.ID)
	}
	return doorDirReal, doorDirAbs, nil
}

func boundContext(ctx context.Context, max time.Duration) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return context.WithTimeout(ctx, max)
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return context.WithTimeout(ctx, 0)
	}
	if remaining < max {
		return context.WithTimeout(ctx, remaining)
	}
	return context.WithTimeout(ctx, max)
}

func stdioTimeout(manifest DoorManifest) time.Duration {
	if manifest.Isolation.TimeoutMS > 0 {
		return time.Duration(manifest.Isolation.TimeoutMS) * time.Millisecond
	}
	return 5 * time.Second
}

func invokeStdioProcess(ctx context.Context, command []string, dir string, env []string, timeout time.Duration, request protocol.RuntimeRequest) (protocol.RuntimeResponse, error) {
	var cancel context.CancelFunc
	ctx, cancel = boundContext(ctx, timeout)
	defer cancel()

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return protocol.RuntimeResponse{}, fmt.Errorf("marshal stdio door request: %w", err)
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(requestBytes)

	stdout := &cappedBuffer{limit: maxStdioStdoutBytes}
	stderr := &cappedBuffer{limit: maxStdioStderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			message := fmt.Sprintf("invoke stdio door: %v", ctx.Err())
			return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeTimeout, message, err)
		}
		diagnostics := strings.TrimSpace(stderr.String())
		if stderr.truncated {
			message := fmt.Sprintf("invoke stdio door: %v: stderr exceeded limit", err)
			return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeResourceLimit, message, err)
		}
		code := classifyProcessStartError(command, err, diagnostics)
		message := fmt.Sprintf("invoke stdio door: %v", err)
		if diagnostics != "" {
			message += ": " + diagnostics
		}
		return protocol.RuntimeResponse{}, protocol.NewCodedError(code, message, err)
	}
	if stdout.truncated {
		return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeResourceLimit, "invoke stdio door: response exceeded limit", nil)
	}

	var response protocol.RuntimeResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		diagnostics := strings.TrimSpace(stderr.String())
		if diagnostics != "" {
			message := fmt.Sprintf("decode stdio door response: %v: stderr: %s", err, diagnostics)
			return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeBadOutput, message, err)
		}
		message := fmt.Sprintf("decode stdio door response: %v", err)
		return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeBadOutput, message, err)
	}
	if response.ContractVersion == "" {
		response.ContractVersion = protocol.RuntimeContractVersion
	}
	if response.Error != nil {
		message := fmt.Sprintf("door runtime error %s: %s", response.Error.Code, response.Error.Message)
		return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorDoorCrashed, message, nil)
	}
	return response, nil
}

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.truncated = true
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *cappedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *cappedBuffer) String() string {
	return b.buf.String()
}
