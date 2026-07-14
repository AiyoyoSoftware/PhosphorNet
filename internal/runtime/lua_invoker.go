package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"phosphornet/internal/protocol"
)

type LuaInvoker struct{}

type luaEffects struct {
	stateOps       []protocol.StateOp
	broadcasts     []protocol.BroadcastEffect
	notifies       []protocol.NotifyEffect
	transitions    []protocol.TransitionEffect
	profileUpdates []protocol.ProfileUpdateEffect
	adminOps       []protocol.AdminOp
	actions        []protocol.ActionEffect
}

func (LuaInvoker) Invoke(ctx context.Context, doorsRoot string, manifest DoorManifest, options RuntimeOptions, request protocol.RuntimeRequest) (protocol.RuntimeResponse, error) {
	entryPath, err := resolveDoorEntryPath(doorsRoot, manifest)
	if err != nil {
		return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorManifestInvalid, err.Error(), err)
	}

	luaCfg := options.withDefaults().Lua.merge(manifest.Sandbox)
	if luaCfg.MaxExecutionMS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(luaCfg.MaxExecutionMS)*time.Millisecond)
		defer cancel()
	}

	L := newSandboxedLuaState(ctx, luaCfg)
	defer L.Close()

	effects := &luaEffects{}
	ctxTable, err := runtimeContextToLua(L, request.Context, effects)
	if err != nil {
		return protocol.RuntimeResponse{}, err
	}
	L.SetGlobal("ctx", ctxTable)
	L.SetGlobal("phosphornet", phosphornetModule(L))

	if err := L.DoFile(entryPath); err != nil {
		message := fmt.Sprintf("load lua door: %v", err)
		return protocol.RuntimeResponse{}, protocol.NewCodedError(protocol.ErrorDoorCrashed, message, err)
	}

	view, err := callLuaLifecycle(L, request, ctxTable)
	if err != nil {
		code := protocol.ErrorDoorCrashed
		if ctx.Err() != nil {
			code = protocol.ErrorRuntimeTimeout
		} else if strings.Contains(strings.ToLower(err.Error()), "memory limit") {
			code = protocol.ErrorRuntimeResourceLimit
		}
		return protocol.RuntimeResponse{}, protocol.NewCodedError(code, err.Error(), err)
	}

	userState := luaValueToAny(L.GetField(L.GetField(ctxTable, "states"), "user"))
	userStateBefore := request.Context.State.User
	stateOps := append([]protocol.StateOp{}, effects.stateOps...)
	if !jsonEqual(userStateBefore, userState) && !hasStateOpForScope(stateOps, protocol.StateScopeUser) {
		stateOps = append(stateOps, protocol.StateOp{
			Scope: protocol.StateScopeUser,
			Op:    protocol.StateOpReplace,
			Value: userState,
		})
	}

	response := protocol.RuntimeResponse{
		ContractVersion: protocol.RuntimeContractVersion,
		View:            view,
		StateOps:        stateOps,
		Broadcasts:      effects.broadcasts,
		Notifies:        effects.notifies,
		Transitions:     effects.transitions,
		ProfileUpdates:  effects.profileUpdates,
		AdminOps:        effects.adminOps,
		Actions:         effects.actions,
	}
	return response, nil
}

func newSandboxedLuaState(ctx context.Context, cfg LuaSandboxConfig) *lua.LState {
	L := lua.NewState(lua.Options{
		SkipOpenLibs:     true,
		CallStackSize:    cfg.CallStackSize,
		RegistrySize:     cfg.RegistrySize,
		RegistryMaxSize:  cfg.RegistryMaxSize,
		RegistryGrowStep: cfg.RegistryGrowStep,
	})
	L.SetContext(ctx)
	if cfg.MaxMemoryKB > 0 {
		L.SetMx(cfg.MaxMemoryKB * 1024)
	}
	openLuaLibraries(L, cfg)
	return L
}

func openLuaLibraries(L *lua.LState, cfg LuaSandboxConfig) {
	profile := normalizeSandboxProfile(cfg.Profile)
	libs := cfg.Libraries
	if len(libs) == 0 {
		switch profile {
		case "standard":
			libs = []string{"base", "coroutine", "table", "string", "math"}
		default:
			libs = []string{"base", "table", "string", "math"}
		}
	}

	for _, name := range libs {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "base":
			lua.OpenBase(L)
		case "coroutine":
			lua.OpenCoroutine(L)
		case "table":
			lua.OpenTable(L)
		case "string":
			lua.OpenString(L)
		case "math":
			lua.OpenMath(L)
		}
	}

	for _, name := range []string{"collectgarbage", "dofile", "loadfile"} {
		L.SetGlobal(name, lua.LNil)
	}
}

func callLuaLifecycle(L *lua.LState, request protocol.RuntimeRequest, ctxTable *lua.LTable) (protocol.UINode, error) {
	fn := L.GetGlobal(string(request.Lifecycle))
	if fn == lua.LNil {
		return protocol.Screen(), nil
	}
	if fn.Type() != lua.LTFunction {
		return protocol.UINode{}, fmt.Errorf("lua lifecycle %q is %s, want function", request.Lifecycle, fn.Type().String())
	}

	args := []lua.LValue{ctxTable}
	if request.Event != nil {
		eventValue, err := goValueToLua(L, request.Event)
		if err != nil {
			return protocol.UINode{}, err
		}
		args = append(args, eventValue)
	}

	top := L.GetTop()
	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, args...); err != nil {
		return protocol.UINode{}, fmt.Errorf("invoke lua lifecycle %q: %w", request.Lifecycle, err)
	}
	ret := L.Get(-1)
	L.SetTop(top)
	if ret == lua.LNil {
		return protocol.Screen(), nil
	}
	return luaTableToUINode(ret)
}

func runtimeContextToLua(L *lua.LState, runtimeCtx protocol.RuntimeContext, effects *luaEffects) (*lua.LTable, error) {
	ctxValue, err := goValueToLua(L, runtimeCtx)
	if err != nil {
		return nil, err
	}
	ctxTable, ok := ctxValue.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("runtime context did not convert to table")
	}
	states, ok := L.GetField(ctxTable, "state").(*lua.LTable)
	if !ok {
		states = L.NewTable()
		L.SetField(ctxTable, "state", states)
	}
	L.SetField(ctxTable, "states", states)
	userState, ok := L.GetField(states, "user").(*lua.LTable)
	if !ok {
		userState = L.NewTable()
		L.SetField(states, "user", userState)
	}
	L.SetField(ctxTable, "state", userState)
	L.SetField(ctxTable, "effects", effectsTable(L, ctxTable, states, effects))
	L.SetField(ctxTable, "store", storeTable(L, ctxTable, states, effects))
	L.SetField(ctxTable, "nav", navigationTable(L, states, effects))
	return ctxTable, nil
}

func effectsTable(L *lua.LState, ctxTable *lua.LTable, states *lua.LTable, effects *luaEffects) *lua.LTable {
	table := L.NewTable()
	L.SetFuncs(table, map[string]lua.LGFunction{
		"set_state": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			scope := protocol.StateScope(L.CheckString(offset))
			key := L.CheckString(offset + 1)
			luaSetState(L, states, effects, scope, key, L.Get(offset+2))
			return 0
		},
		"delete_state": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			scope := protocol.StateScope(L.CheckString(offset))
			key := L.CheckString(offset + 1)
			luaDeleteState(L, states, effects, scope, key)
			return 0
		},
		"clear_state": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			scope := protocol.StateScope(L.CheckString(offset))
			value := L.NewTable()
			L.SetField(states, string(scope), value)
			if scope == protocol.StateScopeUser {
				L.SetField(ctxTable, "state", value)
			}
			effects.stateOps = append(effects.stateOps, protocol.StateOp{Scope: scope, Op: protocol.StateOpClear})
			return 0
		},
		"replace_state": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			scope := protocol.StateScope(L.CheckString(offset))
			value := L.CheckAny(offset + 1)
			luaReplaceState(L, states, effects, scope, value)
			if scope == protocol.StateScopeUser {
				L.SetField(ctxTable, "state", value)
			}
			return 0
		},
		"notify": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			effects.notifies = append(effects.notifies, protocol.NotifyEffect{
				Message:       L.CheckString(offset),
				Level:         optString(L, offset+1, "info"),
				Target:        protocol.NotifyTarget(optString(L, offset+2, string(protocol.NotifyTargetSelf))),
				UserPublicKey: optString(L, offset+3, ""),
			})
			return 0
		},
		"broadcast": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			event := protocol.UIEvent{}
			if err := decodeLuaValue(L.Get(offset), &event); err != nil {
				L.RaiseError("invalid broadcast event: %s", err)
				return 0
			}
			effects.broadcasts = append(effects.broadcasts, protocol.BroadcastEffect{
				Event:         event,
				Scope:         protocol.BroadcastScope(optString(L, offset+1, string(protocol.BroadcastScopeRoom))),
				DoorID:        optString(L, offset+2, ""),
				RoomID:        optString(L, offset+3, ""),
				UserPublicKey: optString(L, offset+4, ""),
			})
			return 0
		},
		"transition": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			effects.transitions = append(effects.transitions, protocol.TransitionEffect{
				Kind:   protocol.TransitionKind(L.CheckString(offset)),
				DoorID: optString(L, offset+1, ""),
				RoomID: optString(L, offset+2, ""),
			})
			return 0
		},
		"update_profile": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			update := protocol.ProfileUpdateEffect{}
			if L.GetTop() >= offset && L.Get(offset) != lua.LNil {
				value := L.CheckString(offset)
				update.DisplayName = &value
			}
			if L.GetTop() >= offset+1 && L.Get(offset+1) != lua.LNil {
				value := L.CheckString(offset + 1)
				update.Bio = &value
			}
			if L.GetTop() >= offset+2 && L.Get(offset+2) != lua.LNil {
				value := L.CheckString(offset + 2)
				update.StatusLine = &value
			}
			effects.profileUpdates = append(effects.profileUpdates, update)
			return 0
		},
		"reset_profile": func(L *lua.LState) int {
			effects.profileUpdates = append(effects.profileUpdates, protocol.ProfileUpdateEffect{Reset: true})
			return 0
		},
		"admin_op": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			op := protocol.AdminOp{}
			if err := decodeLuaValue(L.Get(offset), &op); err != nil {
				L.RaiseError("invalid admin op: %s", err)
				return 0
			}
			effects.adminOps = append(effects.adminOps, op)
			return 0
		},
		"action": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			effect := protocol.ActionEffect{
				RuleID:    L.CheckString(offset),
				RequestID: L.CheckString(offset + 1),
			}
			if L.GetTop() >= offset+2 && L.Get(offset+2) != lua.LNil {
				effect.Input = luaValueToAny(L.Get(offset + 2))
			}
			effects.actions = append(effects.actions, effect)
			return 0
		},
	})
	return table
}

func storeTable(L *lua.LState, ctxTable *lua.LTable, states *lua.LTable, effects *luaEffects) *lua.LTable {
	table := L.NewTable()
	L.SetFuncs(table, map[string]lua.LGFunction{
		"get": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			scope := L.CheckString(offset)
			key := L.CheckString(offset + 1)
			fallback := lua.LNil
			if L.GetTop() >= offset+2 {
				fallback = L.Get(offset + 2)
			}
			scopeTable := luaScopedState(L, states, scope)
			value := L.GetField(scopeTable, key)
			if value == lua.LNil {
				value = fallback
			}
			L.Push(value)
			return 1
		},
		"set": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			luaSetState(L, states, effects, protocol.StateScope(L.CheckString(offset)), L.CheckString(offset+1), L.Get(offset+2))
			return 0
		},
		"delete": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			luaDeleteState(L, states, effects, protocol.StateScope(L.CheckString(offset)), L.CheckString(offset+1))
			return 0
		},
		"clear": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			scope := protocol.StateScope(L.CheckString(offset))
			value := L.NewTable()
			L.SetField(states, string(scope), value)
			if scope == protocol.StateScopeUser {
				L.SetField(ctxTable, "state", value)
			}
			effects.stateOps = append(effects.stateOps, protocol.StateOp{Scope: scope, Op: protocol.StateOpClear})
			return 0
		},
		"replace": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			scope := protocol.StateScope(L.CheckString(offset))
			value := L.CheckAny(offset + 1)
			luaReplaceState(L, states, effects, scope, value)
			if scope == protocol.StateScopeUser {
				L.SetField(ctxTable, "state", value)
			}
			return 0
		},
		"append": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			scope := protocol.StateScope(L.CheckString(offset))
			key := L.CheckString(offset + 1)
			item := L.Get(offset + 2)
			limit := L.OptInt(offset+3, 0)
			scopeTable := luaScopedState(L, states, string(scope))
			items, ok := L.GetField(scopeTable, key).(*lua.LTable)
			if !ok || !luaTableIsArray(items) {
				items = L.NewTable()
			}
			items.Append(item)
			for limit > 0 && items.Len() > limit {
				shiftLuaArray(items)
			}
			luaSetState(L, states, effects, scope, key, items)
			L.Push(items)
			return 1
		},
		"all": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			L.Push(luaScopedState(L, states, L.CheckString(offset)))
			return 1
		},
	})
	return table
}

func navigationTable(L *lua.LState, states *lua.LTable, effects *luaEffects) *lua.LTable {
	const navStackKey = "__nav_stack"
	table := L.NewTable()
	setStack := func(stack *lua.LTable) {
		luaSetState(L, states, effects, protocol.StateScopeUser, navStackKey, stack)
	}
	resetStack := func(view string) *lua.LTable {
		stack := L.NewTable()
		if view != "" {
			stack.Append(lua.LString(view))
		}
		setStack(stack)
		return stack
	}
	L.SetFuncs(table, map[string]lua.LGFunction{
		"current": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			fallback := optString(L, offset, "main")
			stack := luaNavigationStack(L, states)
			if stack.Len() == 0 {
				L.Push(lua.LString(fallback))
				return 1
			}
			L.Push(stack.RawGetInt(stack.Len()))
			return 1
		},
		"push": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			view := L.CheckString(offset)
			stack := luaNavigationStack(L, states)
			if stack.Len() == 0 || stack.RawGetInt(stack.Len()).String() != view {
				stack.Append(lua.LString(view))
			}
			setStack(stack)
			L.Push(lua.LString(view))
			return 1
		},
		"back": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			fallback := optString(L, offset, "main")
			stack := luaNavigationStack(L, states)
			if stack.Len() <= 1 {
				stack = resetStack(fallback)
			} else {
				stack.RawSetInt(stack.Len(), lua.LNil)
				setStack(stack)
			}
			if stack.Len() == 0 {
				L.Push(lua.LString(fallback))
				return 1
			}
			L.Push(stack.RawGetInt(stack.Len()))
			return 1
		},
		"reset": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			view := optString(L, offset, "main")
			resetStack(view)
			L.Push(lua.LString(view))
			return 1
		},
		"handle": func(L *lua.LState) int {
			offset := methodOffset(L, table)
			event, ok := L.Get(offset).(*lua.LTable)
			if !ok {
				L.Push(lua.LFalse)
				return 1
			}
			fallback := optString(L, offset+1, "main")
			action := L.GetField(event, "action").String()
			switch {
			case action == "nav:back":
				stack := luaNavigationStack(L, states)
				if stack.Len() <= 1 {
					resetStack(fallback)
				} else {
					stack.RawSetInt(stack.Len(), lua.LNil)
					setStack(stack)
				}
				L.Push(lua.LTrue)
				return 1
			case strings.HasPrefix(action, "nav:push:"):
				view := strings.TrimPrefix(action, "nav:push:")
				if view != "" {
					stack := luaNavigationStack(L, states)
					if stack.Len() == 0 || stack.RawGetInt(stack.Len()).String() != view {
						stack.Append(lua.LString(view))
					}
					setStack(stack)
					L.Push(lua.LTrue)
					return 1
				}
			case strings.HasPrefix(action, "nav:reset:"):
				view := strings.TrimPrefix(action, "nav:reset:")
				if view == "" {
					view = fallback
				}
				resetStack(view)
				L.Push(lua.LTrue)
				return 1
			}
			L.Push(lua.LFalse)
			return 1
		},
	})
	return table
}

func methodOffset(L *lua.LState, receiver *lua.LTable) int {
	if L.GetTop() > 0 && L.Get(1) == receiver {
		return 2
	}
	return 1
}

func luaScopedState(L *lua.LState, states *lua.LTable, scope string) *lua.LTable {
	scopeTable, ok := L.GetField(states, scope).(*lua.LTable)
	if !ok {
		scopeTable = L.NewTable()
		L.SetField(states, scope, scopeTable)
	}
	return scopeTable
}

func setLuaScopedState(L *lua.LState, states *lua.LTable, scope, key string, value lua.LValue) {
	scopeTable := luaScopedState(L, states, scope)
	L.SetField(scopeTable, key, value)
}

func luaSetState(L *lua.LState, states *lua.LTable, effects *luaEffects, scope protocol.StateScope, key string, value lua.LValue) {
	setLuaScopedState(L, states, string(scope), key, value)
	effects.stateOps = append(effects.stateOps, protocol.StateOp{Scope: scope, Op: protocol.StateOpSet, Key: key, Value: luaValueToAny(value)})
}

func luaDeleteState(L *lua.LState, states *lua.LTable, effects *luaEffects, scope protocol.StateScope, key string) {
	setLuaScopedState(L, states, string(scope), key, lua.LNil)
	effects.stateOps = append(effects.stateOps, protocol.StateOp{Scope: scope, Op: protocol.StateOpDelete, Key: key})
}

func luaReplaceState(L *lua.LState, states *lua.LTable, effects *luaEffects, scope protocol.StateScope, value lua.LValue) {
	L.SetField(states, string(scope), value)
	effects.stateOps = append(effects.stateOps, protocol.StateOp{Scope: scope, Op: protocol.StateOpReplace, Value: luaValueToAny(value)})
}

func luaNavigationStack(L *lua.LState, states *lua.LTable) *lua.LTable {
	userState := luaScopedState(L, states, string(protocol.StateScopeUser))
	stack, ok := L.GetField(userState, "__nav_stack").(*lua.LTable)
	if !ok || !luaTableIsArray(stack) {
		return L.NewTable()
	}
	return stack
}

func shiftLuaArray(table *lua.LTable) {
	for i := 1; i < table.Len(); i++ {
		table.RawSetInt(i, table.RawGetInt(i+1))
	}
	table.RawSetInt(table.Len(), lua.LNil)
}

func phosphornetModule(L *lua.LState) *lua.LTable {
	mod := L.NewTable()
	ui := L.NewTable()
	L.SetFuncs(ui, map[string]lua.LGFunction{
		"screen": func(L *lua.LState) int {
			node := componentTable(L, "screen", "", "", "", L.OptTable(1, L.NewTable()))
			setOptionalStyle(L, node, 2)
			L.Push(node)
			return 1
		},
		"header": func(L *lua.LState) int {
			L.Push(textComponentTable(L, "header", L.CheckString(1)))
			return 1
		},
		"text": func(L *lua.LState) int {
			L.Push(textComponentTable(L, "text", L.CheckString(1)))
			return 1
		},
		"markdown": func(L *lua.LState) int {
			L.Push(textComponentTable(L, "markdown", L.CheckString(1)))
			return 1
		},
		"status": func(L *lua.LState) int {
			L.Push(textComponentTable(L, "status", L.CheckString(1)))
			return 1
		},
		"panel": func(L *lua.LState) int {
			node := componentTable(L, "panel", "", L.CheckString(1), "", L.OptTable(2, L.NewTable()))
			setOptionalStyle(L, node, 3)
			L.Push(node)
			return 1
		},
		"button": func(L *lua.LState) int {
			L.Push(componentTable(L, "button", L.CheckString(1), "", L.CheckString(2), nil, optString(L, 3, "")))
			return 1
		},
		"nav_button": func(L *lua.LState) int {
			L.Push(componentTable(L, "button", L.CheckString(1), "", L.CheckString(2), nil, "nav:push:"+L.CheckString(3)))
			return 1
		},
		"back_button": func(L *lua.LState) int {
			L.Push(componentTable(L, "button", L.CheckString(1), "", optString(L, 2, "Back"), nil, "nav:back"))
			return 1
		},
		"checkbox": func(L *lua.LState) int {
			node := componentTable(L, "checkbox", L.CheckString(1), "", L.CheckString(2), nil, optString(L, 4, ""))
			L.SetField(node, "checked", lua.LBool(L.CheckBool(3)))
			L.Push(node)
			return 1
		},
		"input": func(L *lua.LState) int {
			node := componentTable(L, "input", L.CheckString(1), "", "", nil)
			L.SetField(node, "placeholder", lua.LString(optString(L, 2, "")))
			L.SetField(node, "value", lua.LString(optString(L, 3, "")))
			if dock := optString(L, 4, ""); dock != "" {
				L.SetField(node, "dock", lua.LString(dock))
			}
			L.Push(node)
			return 1
		},
		"textarea": func(L *lua.LState) int {
			node := componentTable(L, "textarea", L.CheckString(1), "", "", nil)
			L.SetField(node, "placeholder", lua.LString(optString(L, 2, "")))
			L.SetField(node, "value", lua.LString(optString(L, 3, "")))
			if dock := optString(L, 4, ""); dock != "" {
				L.SetField(node, "dock", lua.LString(dock))
			}
			L.Push(node)
			return 1
		},
		"grid": func(L *lua.LState) int {
			node := componentTable(L, "grid", L.CheckString(1), "", "", nil)
			L.SetField(node, "rows", L.CheckTable(2))
			L.Push(node)
			return 1
		},
		"log": func(L *lua.LState) int {
			node := componentTable(L, "log", L.CheckString(1), "", "", L.OptTable(2, L.NewTable()))
			setOptionalStyle(L, node, 3)
			L.Push(node)
			return 1
		},
		"menu": func(L *lua.LState) int {
			node := componentTable(L, "menu", L.CheckString(1), "", "", nil)
			L.SetField(node, "items", L.OptTable(2, L.NewTable()))
			L.Push(node)
			return 1
		},
		"list": func(L *lua.LState) int {
			node := componentTable(L, "list", L.CheckString(1), "", "", nil)
			L.SetField(node, "items", L.OptTable(2, L.NewTable()))
			L.Push(node)
			return 1
		},
		"dynamic_list": func(L *lua.LState) int {
			node := componentTable(L, "dynamic_list", L.CheckString(1), "", "", nil)
			L.SetField(node, "items", L.OptTable(2, L.NewTable()))
			L.Push(node)
			return 1
		},
		"item": func(L *lua.LState) int {
			item := L.NewTable()
			L.SetField(item, "label", lua.LString(L.CheckString(1)))
			if action := optString(L, 2, ""); action != "" {
				L.SetField(item, "action", lua.LString(action))
			}
			L.Push(item)
			return 1
		},
	})
	L.SetField(mod, "ui", ui)
	return mod
}

func setOptionalStyle(L *lua.LState, node *lua.LTable, index int) {
	if L.GetTop() < index || L.Get(index) == lua.LNil {
		return
	}
	L.SetField(node, "style", L.CheckTable(index))
}

func textComponentTable(L *lua.LState, component, text string) *lua.LTable {
	node := L.NewTable()
	L.SetField(node, "component", lua.LString(component))
	L.SetField(node, "text", lua.LString(text))
	return node
}

func componentTable(L *lua.LState, component, id, title, text string, children *lua.LTable, action ...string) *lua.LTable {
	node := L.NewTable()
	L.SetField(node, "component", lua.LString(component))
	if id != "" {
		L.SetField(node, "id", lua.LString(id))
	}
	if title != "" {
		L.SetField(node, "title", lua.LString(title))
	}
	if text != "" {
		L.SetField(node, "text", lua.LString(text))
	}
	if len(action) > 0 && action[0] != "" {
		L.SetField(node, "action", lua.LString(action[0]))
	}
	if children != nil {
		L.SetField(node, "children", children)
	}
	return node
}

func optString(L *lua.LState, index int, fallback string) string {
	if L.GetTop() < index || L.Get(index) == lua.LNil {
		return fallback
	}
	return L.CheckString(index)
}

func goValueToLua(L *lua.LState, value any) (lua.LValue, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return lua.LNil, err
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return lua.LNil, err
	}
	return anyToLuaValue(L, decoded), nil
}

func anyToLuaValue(L *lua.LState, value any) lua.LValue {
	switch v := value.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case string:
		return lua.LString(v)
	case float64:
		return lua.LNumber(v)
	case []any:
		table := L.NewTable()
		for _, item := range v {
			table.Append(anyToLuaValue(L, item))
		}
		return table
	case map[string]any:
		table := L.NewTable()
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			L.SetField(table, key, anyToLuaValue(L, v[key]))
		}
		return table
	default:
		return lua.LString(fmt.Sprint(v))
	}
}

func luaTableToUINode(value lua.LValue) (protocol.UINode, error) {
	node := protocol.UINode{}
	if err := decodeLuaValue(value, &node); err != nil {
		return protocol.UINode{}, fmt.Errorf("decode lua UI node: %w", err)
	}
	if node.Component == "" {
		return protocol.UINode{}, fmt.Errorf("lua lifecycle returned UI node with no component")
	}
	return node, nil
}

func decodeLuaValue(value lua.LValue, target any) error {
	data, err := json.Marshal(luaValueToAny(value))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func luaValueToAny(value lua.LValue) any {
	if value == lua.LNil {
		return nil
	}
	switch v := value.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		f := float64(v)
		if math.Trunc(f) == f {
			return int64(f)
		}
		return f
	case lua.LString:
		return string(v)
	case *lua.LTable:
		if luaTableIsArray(v) {
			items := make([]any, 0, v.Len())
			for i := 1; i <= v.Len(); i++ {
				items = append(items, luaValueToAny(v.RawGetInt(i)))
			}
			return items
		}
		result := map[string]any{}
		v.ForEach(func(key lua.LValue, val lua.LValue) {
			result[key.String()] = luaValueToAny(val)
		})
		return result
	default:
		return v.String()
	}
}

func luaTableIsArray(table *lua.LTable) bool {
	if table.Len() == 0 {
		return false
	}
	count := 0
	isArray := true
	table.ForEach(func(key lua.LValue, _ lua.LValue) {
		n, ok := key.(lua.LNumber)
		if !ok || int(n) < 1 || float64(int(n)) != float64(n) {
			isArray = false
			return
		}
		count++
	})
	return isArray && count == table.Len()
}

func jsonEqual(left, right any) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func hasStateOpForScope(ops []protocol.StateOp, scope protocol.StateScope) bool {
	for _, op := range ops {
		if op.Scope == scope {
			return true
		}
	}
	return false
}
