import asyncio
import copy
import importlib.util
import json
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from phosphornet.ctx import DoorContext


CONTRACT_VERSION = "phosphornet.door.runtime.v1"


def load_module(path: str):
    spec = importlib.util.spec_from_file_location("door_app", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"unable to load door module: {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _empty_view():
    return {"component": "screen", "children": []}


def _state_ops_from_legacy_mutation(before: dict, after: dict):
    if before == after:
        return []
    return [{"scope": "user", "op": "replace", "value": after}]


async def _invoke_namespace(namespace: dict, request: dict):
    lifecycle = request["lifecycle"]
    fn = namespace.get(lifecycle)
    runtime_ctx = request.get("ctx", {})
    states = runtime_ctx.get("state", {})
    user_state_before = copy.deepcopy(states.get("user", {}))
    door_ctx = DoorContext(
        session=runtime_ctx.get("session", {}),
        user=runtime_ctx.get("user", {}),
        node=runtime_ctx.get("node", {}),
        room=runtime_ctx.get("room", {}),
        states=states,
        settings=runtime_ctx.get("settings", {}),
        presence=runtime_ctx.get("presence", {}),
        users=runtime_ctx.get("users", []),
        storage=runtime_ctx.get("storage", {}),
        events=runtime_ctx.get("events", []),
        admin=runtime_ctx.get("admin", {}),
        permissions=runtime_ctx.get("permissions", {}),
    )

    if fn is None:
        view = _empty_view()
    elif request.get("event") is None:
        view = await fn(door_ctx)
    else:
        view = await fn(door_ctx, request.get("event"))

    return {
        "contract_version": CONTRACT_VERSION,
        "view": view,
        "state_ops": door_ctx.effects.state_ops + _state_ops_from_legacy_mutation(user_state_before, door_ctx.state),
        "broadcasts": door_ctx.effects.broadcasts,
        "notifies": door_ctx.effects.notifies,
        "transitions": door_ctx.effects.transitions,
        "profile_updates": door_ctx.effects.profile_updates,
        "admin_ops": door_ctx.effects.admin_ops,
        "actions": door_ctx.effects.actions,
    }


async def invoke(entry_path: str, request: dict):
    module = load_module(entry_path)
    return await _invoke_namespace(module.__dict__, request)


async def run_module(namespace: dict):
    request = json.loads(sys.stdin.read())
    result = await _invoke_namespace(namespace, request)
    print(json.dumps(result))


async def main():
    request = json.loads(sys.stdin.read())
    entry_path = os.environ.get("PHOSPHORNET_DOOR_ENTRY")
    if not entry_path and "entry_path" in request and "request" in request:
        entry_path = request["entry_path"]
        request = request["request"]
    if not entry_path:
        raise RuntimeError("PHOSPHORNET_DOOR_ENTRY is required")
    result = await invoke(
        entry_path,
        request,
    )
    print(json.dumps(result))


if __name__ == "__main__":
    asyncio.run(main())
