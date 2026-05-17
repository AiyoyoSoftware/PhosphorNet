package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"phosphornet/internal/protocol"
)

type DoorResponse = protocol.RuntimeResponse

type Invoker interface {
	Invoke(ctx context.Context, doorsRoot string, manifest DoorManifest, options RuntimeOptions, request protocol.RuntimeRequest) (protocol.RuntimeResponse, error)
}

var invokers = map[string]Invoker{
	"stdio": StdioInvoker{},
	"lua":   LuaInvoker{},
}

func InvokeDoorLifecycle(ctx context.Context, doorsRoot string, manifest DoorManifest, request protocol.RuntimeRequest) (protocol.RuntimeResponse, error) {
	return InvokeDoorLifecycleWithOptions(ctx, doorsRoot, manifest, DefaultRuntimeOptions(), request)
}

func InvokeDoorLifecycleWithOptions(ctx context.Context, doorsRoot string, manifest DoorManifest, options RuntimeOptions, request protocol.RuntimeRequest) (protocol.RuntimeResponse, error) {
	options = options.withDefaults()
	invoker, err := resolveInvokerWithOptions(manifest, options)
	if err != nil {
		return protocol.RuntimeResponse{}, err
	}
	if request.ContractVersion == "" {
		request.ContractVersion = protocol.RuntimeContractVersion
	}
	if request.Door.ID == "" {
		request.Door = protocol.RuntimeDoor{ID: manifest.ID, Name: manifest.Name}
	}
	response, err := invoker.Invoke(ctx, doorsRoot, manifest, options, request)
	if err != nil {
		return protocol.RuntimeResponse{}, err
	}
	if err := protocol.ValidateRuntimeResponse(response); err != nil {
		message := fmt.Sprintf("validate door response: %v", err)
		return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorRuntimeBadOutput, message, err)
	}
	return response, nil
}

func InvokeDoorView(ctx context.Context, doorsRoot string, manifest DoorManifest, runtimeCtx protocol.RuntimeContext) (protocol.RuntimeResponse, error) {
	return InvokeDoorLifecycle(ctx, doorsRoot, manifest, protocol.RuntimeRequest{
		Lifecycle: protocol.LifecycleView,
		Context:   runtimeCtx,
	})
}

func InvokeDoorUpdate(ctx context.Context, doorsRoot string, manifest DoorManifest, runtimeCtx protocol.RuntimeContext, event protocol.UIEvent) (protocol.RuntimeResponse, error) {
	return InvokeDoorLifecycle(ctx, doorsRoot, manifest, protocol.RuntimeRequest{
		Lifecycle: protocol.LifecycleUpdate,
		Context:   runtimeCtx,
		Event:     &event,
	})
}

func InvokeDoorHook(ctx context.Context, doorsRoot string, manifest DoorManifest, lifecycle protocol.Lifecycle, runtimeCtx protocol.RuntimeContext) (protocol.RuntimeResponse, error) {
	switch lifecycle {
	case protocol.LifecycleInit, protocol.LifecycleOnJoin, protocol.LifecycleOnLeave, protocol.LifecycleTick:
	default:
		return protocol.RuntimeResponse{}, fmt.Errorf("unsupported door hook lifecycle %q", lifecycle)
	}
	return InvokeDoorLifecycle(ctx, doorsRoot, manifest, protocol.RuntimeRequest{
		Lifecycle: lifecycle,
		Context:   runtimeCtx,
	})
}

func resolveInvoker(manifest DoorManifest) (Invoker, error) {
	return resolveInvokerWithOptions(manifest, DefaultRuntimeOptions())
}

func resolveInvokerWithOptions(manifest DoorManifest, options RuntimeOptions) (Invoker, error) {
	name := normalizeRuntimeNameWithOptions(manifest, options)
	invoker, ok := invokers[name]
	if !ok {
		return nil, protocol.NewCodedError(protocol.ErrorRuntimeNotAvailable, fmt.Sprintf("no invoker registered for runtime %q", name), nil)
	}
	return invoker, nil
}

func normalizeRuntimeName(manifest DoorManifest) string {
	return normalizeRuntimeNameWithOptions(manifest, DefaultRuntimeOptions())
}

func normalizeRuntimeNameWithOptions(manifest DoorManifest, options RuntimeOptions) string {
	if manifest.Runtime != "" {
		return strings.ToLower(manifest.Runtime)
	}
	switch strings.ToLower(filepath.Ext(manifest.Entry)) {
	case ".lua":
		return "lua"
	default:
		defaultRuntime := options.withDefaults().DefaultRuntime
		if defaultRuntime == "" {
			return "lua"
		}
		return strings.ToLower(defaultRuntime)
	}
}
