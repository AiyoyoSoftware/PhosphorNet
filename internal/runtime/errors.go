package runtime

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"phosphornet/internal/protocol"
)

func protocolManifestError(err error) error {
	return protocol.NewCodedError(protocol.ErrorManifestInvalid, err.Error(), err)
}

func classifyProcessStartError(command []string, err error, stderr string) protocol.ErrorCode {
	executable := ""
	if len(command) > 0 {
		executable = command[0]
	}
	diagnostics := strings.ToLower(stderr)
	switch {
	case errors.Is(err, exec.ErrNotFound):
		if executable == "podman" {
			return protocol.ErrorRuntimeNotAvailable
		}
		return protocol.ErrorDoorCrashed
	case errors.Is(err, os.ErrPermission), strings.Contains(diagnostics, "permission denied"):
		return protocol.ErrorRuntimeDeniedPolicy
	case executable == "podman" && podmanDiagnosticsLookLikeMissingImage(diagnostics):
		return protocol.ErrorRuntimeImageMissing
	default:
		return protocol.ErrorDoorCrashed
	}
}

func podmanDiagnosticsLookLikeMissingImage(diagnostics string) bool {
	for _, needle := range []string{
		"no such image",
		"image not known",
		"manifest unknown",
		"repository does not exist",
		"not found",
		"unable to find",
		"pull access denied",
	} {
		if strings.Contains(diagnostics, needle) {
			return true
		}
	}
	return false
}
