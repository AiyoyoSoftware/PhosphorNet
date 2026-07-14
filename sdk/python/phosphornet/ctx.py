from dataclasses import dataclass, field


class DoorStore:
    def __init__(self, owner):
        self.owner = owner

    def _scope(self, scope: str) -> dict:
        return self.owner.states.setdefault(scope, {})

    def get(self, scope: str, key: str, default=None):
        return self._scope(scope).get(key, default)

    def set(self, scope: str, key: str, value):
        self.owner.effects.set_state(scope, key, value)

    def delete(self, scope: str, key: str):
        self.owner.effects.delete_state(scope, key)

    def clear(self, scope: str):
        self.owner.effects.clear_state(scope)

    def replace(self, scope: str, value: dict):
        self.owner.effects.replace_state(scope, value)

    def append(self, scope: str, key: str, value, limit: int | None = None):
        items = list(self.get(scope, key, []))
        items.append(value)
        if limit is not None and limit > 0:
            items = items[-limit:]
        self.set(scope, key, items)
        return items

    def all(self, scope: str) -> dict:
        return self._scope(scope)


class DoorNavigation:
    _STACK_KEY = "__nav_stack"

    def __init__(self, owner):
        self.owner = owner

    def _stack(self, create: bool = False) -> list:
        stack = self.owner.state.get(self._STACK_KEY)
        if not isinstance(stack, list):
            if not create:
                return []
            stack = []
            self.owner.state[self._STACK_KEY] = stack
        return stack

    def current(self, default: str = "main") -> str:
        stack = self._stack()
        if not stack:
            return default
        return str(stack[-1])

    def push(self, view: str) -> str:
        stack = self._stack(create=True)
        if not stack or stack[-1] != view:
            stack.append(view)
        self.owner.effects.set_state("user", self._STACK_KEY, stack)
        return view

    def back(self, default: str = "main") -> str:
        stack = self._stack(create=True)
        if len(stack) <= 1:
            stack = [default]
        else:
            stack = stack[:-1]
        self.owner.effects.set_state("user", self._STACK_KEY, stack)
        return str(stack[-1]) if stack else default

    def reset(self, view: str = "main") -> str:
        stack = [view] if view else []
        self.owner.effects.set_state("user", self._STACK_KEY, stack)
        return view

    def handle(self, event: dict | None, default: str = "main") -> bool:
        if not event:
            return False
        action = event.get("action", "")
        if action == "nav:back":
            self.back(default)
            return True
        if action.startswith("nav:push:"):
            view = action.removeprefix("nav:push:")
            if view:
                self.push(view)
                return True
        if action.startswith("nav:reset:"):
            view = action.removeprefix("nav:reset:") or default
            self.reset(view)
            return True
        return False


class DoorEffects:
    def __init__(self, owner=None):
        self.owner = owner
        self.state_ops: list[dict] = []
        self.broadcasts: list[dict] = []
        self.notifies: list[dict] = []
        self.transitions: list[dict] = []
        self.profile_updates: list[dict] = []
        self.admin_ops: list[dict] = []
        self.actions: list[dict] = []

    def set_state(self, scope: str, key: str, value):
        if self.owner is not None:
            self.owner.states.setdefault(scope, {})[key] = value
        self.state_ops.append({"scope": scope, "op": "set", "key": key, "value": value})

    def delete_state(self, scope: str, key: str):
        if self.owner is not None:
            self.owner.states.setdefault(scope, {}).pop(key, None)
        self.state_ops.append({"scope": scope, "op": "delete", "key": key})

    def clear_state(self, scope: str):
        if self.owner is not None:
            self.owner.states[scope] = {}
        self.state_ops.append({"scope": scope, "op": "clear"})

    def replace_state(self, scope: str, value: dict):
        if self.owner is not None:
            self.owner.states[scope] = value
        self.state_ops.append({"scope": scope, "op": "replace", "value": value})

    def notify(self, message: str, level: str = "info", target: str = "self", user_public_key: str = ""):
        effect = {"target": target, "level": level, "message": message}
        if user_public_key:
            effect["user_public_key"] = user_public_key
        self.notifies.append(effect)

    def broadcast(self, event: dict, scope: str = "room", door_id: str = "", room_id: str = "", user_public_key: str = ""):
        effect = {"scope": scope, "event": event}
        if door_id:
            effect["door_id"] = door_id
        if room_id:
            effect["room_id"] = room_id
        if user_public_key:
            effect["user_public_key"] = user_public_key
        self.broadcasts.append(effect)

    def transition(self, kind: str, door_id: str = "", room_id: str = ""):
        effect = {"kind": kind}
        if door_id:
            effect["door_id"] = door_id
        if room_id:
            effect["room_id"] = room_id
        self.transitions.append(effect)

    def update_profile(self, display_name=None, bio=None, status_line=None):
        effect = {}
        if display_name is not None:
            effect["display_name"] = display_name
        if bio is not None:
            effect["bio"] = bio
        if status_line is not None:
            effect["status_line"] = status_line
        self.profile_updates.append(effect)

    def reset_profile(self):
        self.profile_updates.append({"reset": True})

    def admin_op(self, op: dict):
        self.admin_ops.append(dict(op))

    def action(self, rule_id: str, request_id: str, input=None):
        effect = {"rule_id": rule_id, "request_id": request_id}
        if input is not None:
            effect["input"] = input
        self.actions.append(effect)


@dataclass
class DoorContext:
    session: dict = field(default_factory=dict)
    user: dict = field(default_factory=dict)
    node: dict = field(default_factory=dict)
    room: dict = field(default_factory=dict)
    states: dict = field(default_factory=dict)
    settings: dict = field(default_factory=dict)
    presence: dict = field(default_factory=dict)
    users: list = field(default_factory=list)
    storage: dict = field(default_factory=dict)
    events: list = field(default_factory=list)
    admin: dict = field(default_factory=dict)
    permissions: dict = field(default_factory=dict)
    effects: DoorEffects = field(default_factory=DoorEffects)
    store: DoorStore = field(init=False)
    nav: DoorNavigation = field(init=False)

    def __post_init__(self):
        self.effects = DoorEffects(self)
        self.states.setdefault("user", {})
        self.states.setdefault("room", {})
        self.states.setdefault("global", {})
        self.store = DoorStore(self)
        self.nav = DoorNavigation(self)

    @property
    def state(self):
        return self.states.setdefault("user", {})
