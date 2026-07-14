//go:build !linux

package action

import (
	"context"
	"os/exec"
)

func newCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
