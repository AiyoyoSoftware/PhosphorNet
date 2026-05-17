package runtime

import (
	"context"
	"strconv"
	"strings"

	"phosphornet/internal/protocol"
)

const (
	defaultPodmanImageWorkdir = "/door"
	defaultPodmanMemory       = "128m"
	defaultPodmanCPUs         = 0.25
	defaultPodmanPidsLimit    = 64
)

type ContainerInvoker struct{}

func (ContainerInvoker) Invoke(ctx context.Context, doorsRoot string, manifest DoorManifest, _ RuntimeOptions, request protocol.RuntimeRequest) (protocol.RuntimeResponse, error) {
	if _, err := resolveDoorDir(doorsRoot, manifest); err != nil {
		return protocol.RuntimeResponse{}, err
	}
	command, err := buildPodmanRunCommand(manifest)
	if err != nil {
		return protocol.RuntimeResponse{}, err
	}
	return invokeStdioProcess(ctx, command, "", nil, stdioTimeout(manifest), request)
}

func buildPodmanRunCommand(manifest DoorManifest) ([]string, error) {
	isolation := manifest.Isolation
	mode := strings.ToLower(strings.TrimSpace(isolation.Mode))
	if mode == "" {
		mode = IsolationModePodman
	}
	if mode != IsolationModePodman {
		message := "podman invoker requires podman isolation mode"
		return nil, protocol.NewCodedError(protocol.ErrorRuntimeDeniedPolicy, message, nil)
	}
	image := strings.TrimSpace(isolation.Image)
	if image == "" {
		message := "podman isolation requires image"
		return nil, protocol.NewCodedError(protocol.ErrorRuntimeImageMissing, message, nil)
	}

	network := strings.ToLower(strings.TrimSpace(isolation.Network))
	if network == "" {
		network = IsolationNetworkNone
	}
	memory := strings.TrimSpace(isolation.Memory)
	if memory == "" {
		memory = defaultPodmanMemory
	}
	cpus := isolation.CPUs
	if cpus == 0 {
		cpus = defaultPodmanCPUs
	}
	pidsLimit := isolation.PidsLimit
	if pidsLimit == 0 {
		pidsLimit = defaultPodmanPidsLimit
	}

	args := []string{
		"podman",
		"run",
		"--rm",
		"-i",
		"--network=" + network,
		"--memory=" + memory,
		"--cpus=" + strconv.FormatFloat(cpus, 'f', -1, 64),
		"--pids-limit=" + strconv.Itoa(pidsLimit),
		"--security-opt=no-new-privileges",
		"--cap-drop=ALL",
		"--userns=keep-id",
		"--workdir=" + defaultPodmanImageWorkdir,
	}
	if isolation.ReadOnly == nil || *isolation.ReadOnly {
		args = append(args, "--read-only")
	}
	args = append(args, image)
	args = append(args, manifest.Command...)
	return args, nil
}
