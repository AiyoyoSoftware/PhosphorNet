local ui = phosphornet.ui

local function admin_ctx(ctx)
  return ctx.admin or {}
end

local function global_state(ctx)
  return admin_ctx(ctx).policy or ctx.states.global or {}
end

local function maintenance_count(ctx)
  return tonumber(global_state(ctx).maintenance_count or 0) or 0
end

local function maintenance_mode(ctx)
  if global_state(ctx).maintenance_mode then
    return "enabled"
  end
  return "disabled"
end

local function draft_notice(ctx)
  return tostring(ctx.state.station_notice or "")
end

local function station_roles(ctx)
  return global_state(ctx).roles or {}
end

local function door_role_policy(ctx)
  return global_state(ctx).door_roles or {}
end

local function disabled_doors(ctx)
  return global_state(ctx).disabled_doors or {}
end

local function door_order(ctx)
  return global_state(ctx).door_order or {}
end

local function door_settings_state(ctx)
  return global_state(ctx).door_settings or {}
end

local function moderation_policy(ctx)
  return global_state(ctx).moderation or {}
end

local function banned_keys(ctx)
  return moderation_policy(ctx).banned_keys or {}
end

local function muted_keys(ctx)
  return moderation_policy(ctx).muted_keys or {}
end

local function user_rate_limits(ctx)
  return moderation_policy(ctx).rate_limits or {}
end

local function moderation_notes(ctx)
  return moderation_policy(ctx).notes or {}
end

local function admin_doors(ctx)
  return admin_ctx(ctx).doors or ctx.node.doors or {}
end

local function admin_users(ctx)
  return admin_ctx(ctx).users or ctx.users or {}
end

local function admin_storage(ctx)
  return admin_ctx(ctx).storage or ctx.storage or {}
end

local function admin_events(ctx)
  return admin_ctx(ctx).events or ctx.events or {}
end

local function admin_op(ctx, op)
  ctx.effects.admin_op(op)
end

local function admin_doors_dir(ctx)
  return admin_ctx(ctx).doors_dir or ctx.node.doors_dir
end

local function admin_database_path(ctx)
  return (admin_storage(ctx) and admin_storage(ctx).database_path) or admin_ctx(ctx).database_path or ctx.node.database_path
end

local function admin_default_runtime(ctx)
  return admin_ctx(ctx).default_runtime or ctx.node.default_runtime
end

local function admin_lua_sandbox(ctx)
  return admin_ctx(ctx).lua_sandbox or ctx.node.lua_sandbox or {}
end

local function admin_station_allowlist(ctx)
  return admin_ctx(ctx).station_allowlist or ctx.node.station_allowlist or {}
end

local function admin_admins(ctx)
  return admin_ctx(ctx).admins or ctx.node.admins or {}
end

local function setting(ctx, key, fallback)
  if ctx.settings and ctx.settings[key] ~= nil then
    return ctx.settings[key]
  end
  return fallback
end

local function gradient_background(direction, stops)
  return {
    background = {
      kind = "gradient",
      direction = direction,
      stops = stops,
    },
  }
end

local admin_panel_palettes = {
  {
    direction = "vertical",
    stops = {
      { at = 0.0, color = "#101827" },
      { at = 0.52, color = "#172033" },
      { at = 1.0, color = "#1f2b24" },
    },
  },
  {
    direction = "diagonal",
    stops = {
      { at = 0.0, color = "#111827" },
      { at = 0.48, color = "#1d2432" },
      { at = 1.0, color = "#26301f" },
    },
  },
  {
    direction = "horizontal",
    stops = {
      { at = 0.0, color = "#0f1b2d" },
      { at = 0.5, color = "#192235" },
      { at = 1.0, color = "#202c25" },
    },
  },
  {
    direction = "vertical",
    stops = {
      { at = 0.0, color = "#151827" },
      { at = 0.55, color = "#1f2230" },
      { at = 1.0, color = "#263127" },
    },
  },
}

local function admin_panel_style(title)
  local value = tostring(title or "")
  local sum = 0
  for index = 1, #value do
    sum = sum + string.byte(value, index)
  end
  local palette = admin_panel_palettes[(sum % #admin_panel_palettes) + 1]
  return gradient_background(palette.direction, palette.stops)
end

local function admin_panel(title, children, style)
  return ui.panel(title, children, style or admin_panel_style(title))
end

local function int_setting(ctx, key, fallback, minimum)
  local value = tonumber(setting(ctx, key, fallback)) or fallback
  minimum = minimum or 1
  if value < minimum then
    return minimum
  end
  return math.floor(value)
end

local function copy_table(source)
  local result = {}
  for key, value in pairs(source or {}) do
    result[key] = value
  end
  return result
end

local function trim(value)
  return tostring(value or ""):match("^%s*(.-)%s*$")
end

local function short_key(value)
  value = tostring(value or "")
  if #value <= 18 then
    return value
  end
  return value:sub(1, 10) .. "..." .. value:sub(-6)
end

local function command_text(command)
  local parts = {}
  for _, value in ipairs(command or {}) do
    table.insert(parts, tostring(value))
  end
  if #parts == 0 then
    return "none"
  end
  return table.concat(parts, " ")
end

local function has_prefix(value, prefix)
  return tostring(value or ""):sub(1, #prefix) == prefix
end

local function strip_prefix(value, prefix)
  return tostring(value or ""):sub(#prefix + 1)
end

local function guest_name(fingerprint)
  local value = tostring(fingerprint or "unknown")
  local first = value:match("^[^-]+") or value
  return "guest-" .. first
end

local function display_name(user)
  local name = trim(user and user.display_name or "")
  if name ~= "" then
    return name
  end
  return guest_name(user and user.fingerprint)
end

local function split_roles(value)
  local roles = {}
  local seen = {}
  for part in tostring(value or ""):gmatch("[^,]+") do
    local role = trim(part):lower()
    if role ~= "" and not seen[role] then
      seen[role] = true
      table.insert(roles, role)
    end
  end
  table.sort(roles)
  return roles
end

local function join_roles(roles)
  local values = {}
  for _, role in ipairs(roles or {}) do
    table.insert(values, tostring(role))
  end
  return table.concat(values, ", ")
end

local function event_value(event, key, fallback)
  if event and event.values and event.values[key] and event.values[key] ~= "" then
    return event.values[key]
  end
  return fallback or ""
end

local function current_page(ctx)
  return ctx.nav:current("home")
end

local function selected_nav_door(ctx)
  return trim(ctx.state.selected_nav_door or "")
end

local door_by_id
local append_notice

local function count_map(values)
  local count = 0
  for _ in pairs(values or {}) do
    count = count + 1
  end
  return count
end

local function setting_label(setting)
  local label = trim(setting and setting.label or "")
  if label ~= "" then
    return label
  end
  return tostring(setting and setting.name or "setting")
end

local function setting_input_id(door_id, name)
  return "setting-" .. tostring(door_id or "unknown") .. "-" .. tostring(name or "unknown")
end

local function setting_value(setting)
  if setting and setting.value ~= nil then
    return setting.value
  end
  return setting and setting.default
end

local function setting_value_text(setting)
  local value = setting_value(setting)
  if value == nil then
    return ""
  end
  return tostring(value)
end

local function setting_choices_text(setting)
  local options = {}
  for _, option in ipairs((setting and setting.options) or {}) do
    table.insert(options, tostring(option))
  end
  if #options == 0 then
    return ""
  end
  return table.concat(options, ", ")
end

local function setting_action_suffix(action)
  local door_id, name = tostring(action or ""):match("^[^:]+:([^:]+):(.+)$")
  return trim(door_id), trim(name)
end

local function setting_by_name(door, name)
  for _, setting in ipairs((door and door.settings) or {}) do
    if setting.name == name then
      return setting
    end
  end
  return nil
end

local function setting_option_allowed(setting, value)
  for _, option in ipairs((setting and setting.options) or {}) do
    if tostring(option) == tostring(value) then
      return true
    end
  end
  return false
end

local function parse_setting_value(setting, raw)
  local kind = tostring(setting and setting.type or "string")
  if kind == "bool" then
    return raw == true or tostring(raw) == "true", nil
  elseif kind == "int" then
    local number = tonumber(trim(raw))
    if not number or number ~= math.floor(number) then
      return nil, "Enter a whole number."
    end
    return number, nil
  elseif kind == "select" then
    local value = trim(raw)
    if not setting_option_allowed(setting, value) then
      return nil, "Choose one of: " .. setting_choices_text(setting)
    end
    return value, nil
  end
  return tostring(raw or ""), nil
end

local function save_door_setting(ctx, door_id, name, raw)
  local door = door_by_id(ctx, door_id)
  local setting = setting_by_name(door, name)
  if not door or not setting then
    ctx.effects.notify("Unknown door setting.", "warning", "self")
    return
  end
  local value, err = parse_setting_value(setting, raw)
  if err then
    ctx.effects.notify(err, "warning", "self")
    return
  end
  admin_op(ctx, { op = "set_door_setting", door_id = door_id, setting_key = name, setting_value = value })
  append_notice(ctx, "Updated setting " .. door_id .. "." .. name)
  ctx.effects.notify("Updated " .. door_id .. "." .. name .. ".", "info", "self")
end

local function reset_door_setting(ctx, door_id, name)
  admin_op(ctx, { op = "set_door_setting", door_id = door_id, setting_key = name, reset = true })
  append_notice(ctx, "Reset setting " .. door_id .. "." .. name .. " to manifest default")
  ctx.effects.notify("Setting reset to manifest default.", "info", "self")
end

append_notice = function(ctx, notice)
  admin_op(ctx, { op = "set_station_notice", message = tostring(notice or "") })
end

local function page_shell(ctx, title, body)
  local children = {
    ui.header("STATION ADMIN / " .. title),
    admin_panel("Console", {
      ui.nav_button("admin-home", "Home", "home"),
      ui.nav_button("admin-doors", "Doors", "doors"),
      ui.nav_button("admin-settings", "Door Settings", "settings"),
      ui.nav_button("admin-users", "Users", "users"),
      ui.nav_button("admin-access", "Access Control", "access"),
      ui.nav_button("admin-moderation", "Moderation", "moderation"),
      ui.nav_button("admin-storage", "Storage", "storage"),
      ui.nav_button("admin-runtime", "Runtime", "runtime"),
      ui.nav_button("admin-logs", "Logs", "logs"),
      ui.nav_button("admin-maintenance", "Maintenance", "maintenance"),
    }, gradient_background("horizontal", {
      { at = 0.0, color = "#101827" },
      { at = 0.5, color = "#172033" },
      { at = 1.0, color = "#1f2b24" },
    })),
  }
  for _, node in ipairs(body) do
    table.insert(children, node)
  end
  table.insert(children, ui.status("Admin/sysop only. Dangerous actions should move through confirmation views."))
  return ui.screen(children, gradient_background("diagonal", {
    { at = 0.0, color = "#080f1f" },
    { at = 0.45, color = "#111827" },
    { at = 1.0, color = "#17251f" },
  }))
end

local function enabled_door_items(ctx)
  local items = {}
  for _, door in ipairs(admin_doors(ctx) or {}) do
    if not door.disabled then
      table.insert(items, door)
    end
  end
  return items
end

local function sorted_doors(ctx, enabled_only)
  local order = {}
  for index, door_id in ipairs(door_order(ctx) or {}) do
    order[tostring(door_id)] = index
  end
  local doors = {}
  for _, door in ipairs(admin_doors(ctx) or {}) do
    if not enabled_only or not door.disabled then
      table.insert(doors, door)
    end
  end
  table.sort(doors, function(left, right)
    local left_id = tostring(left.id or "")
    local right_id = tostring(right.id or "")
    local left_order = order[left_id]
    local right_order = order[right_id]
    if left_order and right_order then
      return left_order < right_order
    elseif left_order then
      return true
    elseif right_order then
      return false
    end
    return left_id < right_id
  end)
  return doors
end

local function ordered_door_ids(ctx)
  local ids = {}
  for _, door in ipairs(sorted_doors(ctx, false)) do
    table.insert(ids, tostring(door.id or ""))
  end
  return ids
end

local function move_enabled_door_order(ctx, door_id, delta)
  door_id = trim(door_id)
  if door_id == "" then
    return false, "Select an enabled door first."
  end

  local enabled = enabled_door_items(ctx)
  local enabled_ids = {}
  local selected_index = nil
  for index, door in ipairs(enabled) do
    local id = tostring(door.id or "")
    table.insert(enabled_ids, id)
    if id == door_id then
      selected_index = index
    end
  end
  if not selected_index then
    return false, "Selected door is not currently enabled."
  end

  local target_index = selected_index + delta
  if target_index < 1 or target_index > #enabled_ids then
    return false, delta < 0 and "Door is already first in the enabled navigation order." or "Door is already last in the enabled navigation order."
  end

  enabled_ids[selected_index], enabled_ids[target_index] = enabled_ids[target_index], enabled_ids[selected_index]

  local disabled = disabled_doors(ctx)
  local merged = {}
  local enabled_cursor = 1
  for _, id in ipairs(ordered_door_ids(ctx)) do
    if disabled[id] then
      table.insert(merged, id)
    else
      table.insert(merged, enabled_ids[enabled_cursor])
      enabled_cursor = enabled_cursor + 1
    end
  end
  admin_op(ctx, { op = "reorder_doors", door_order = merged })
  return true, nil
end

door_by_id = function(ctx, door_id)
  for _, door in ipairs(admin_doors(ctx) or {}) do
    if door.id == door_id then
      return door
    end
  end
  return nil
end

local function user_by_fingerprint(ctx, fingerprint)
  for _, user in ipairs(admin_users(ctx) or {}) do
    if user.fingerprint == fingerprint then
      return user
    end
  end
  return nil
end

local function state_counts(ctx)
  local counts = { user = 0, room = 0, global = 0 }
  local bytes = { user = 0, room = 0, global = 0 }
  local doors = {}
  for _, record in ipairs((admin_storage(ctx) and admin_storage(ctx).state_records) or {}) do
    counts[record.scope] = (counts[record.scope] or 0) + 1
    bytes[record.scope] = (bytes[record.scope] or 0) + (tonumber(record.bytes or 0) or 0)
    doors[record.door_id or "unknown"] = true
  end
  return counts, bytes, doors
end

local function hidden_private_counts(ctx)
  local hidden = 0
  local private = 0
  for _, door in ipairs(admin_doors(ctx) or {}) do
    if door.visibility == "hidden" then
      hidden = hidden + 1
    elseif door.visibility == "private" then
      private = private + 1
    end
  end
  return hidden, private
end

local function role_lines(ctx)
  local roles = station_roles(ctx)
  local lines = {}
  for public_key, role in pairs(roles) do
    table.insert(lines, "- " .. tostring(role) .. ": " .. short_key(public_key))
  end
  table.sort(lines)
  if #lines == 0 then
    return { ui.text("No station role assignments.") }
  end

  local nodes = {}
  for _, line in ipairs(lines) do
    table.insert(nodes, ui.text(line))
  end
  return nodes
end

local function notice_lines(ctx)
  local notices = global_state(ctx).notices or {}
  if #notices == 0 then
    return { ui.text("No admin events recorded yet.") }
  end

  local lines = {}
  local limit = int_setting(ctx, "recent_notice_limit", 12, 1)
  local first = math.max(1, #notices - limit + 1)
  for index = first, #notices do
    table.insert(lines, ui.text(tostring(index) .. ". " .. tostring(notices[index])))
  end
  return lines
end

local function event_lines(ctx)
  local events = admin_events(ctx) or {}
  if #events == 0 then
    return { ui.text("No station events recorded in memory yet.") }
  end
  local nodes = {}
  local limit = int_setting(ctx, "recent_event_limit", 15, 1)
  local first = math.max(1, #events - limit + 1)
  for index = first, #events do
    local event = events[index] or {}
    local line = (event.time or "unknown") .. " [" .. (event.type or "event") .. "]"
    if event.door_id and event.door_id ~= "" then
      line = line .. " door=" .. event.door_id
    end
    if event.fingerprint and event.fingerprint ~= "" then
      line = line .. " user=" .. event.fingerprint
    end
    line = line .. " - " .. (event.message or "")
    table.insert(nodes, ui.text(line))
  end
  return nodes
end

local function render_home(ctx)
  local counts = state_counts(ctx)
  local hidden, private = hidden_private_counts(ctx)
  return page_shell(ctx, "HOME", {
    admin_panel("Station Status", {
      ui.text("Station: " .. (ctx.node.name or "unknown")),
      ui.text("Node ID: " .. short_key(ctx.node.id)),
      ui.text("Node fingerprint: " .. (ctx.node.fingerprint or "unknown")),
      ui.text("Signed in as: " .. display_name(ctx.user) .. " · " .. (ctx.user.fingerprint or "unknown")),
      ui.text("Role: " .. (ctx.user.role or "member")),
      ui.text("Access mode: " .. (ctx.node.access_mode or "public")),
      ui.text("Runtime: default " .. (admin_default_runtime(ctx) or "unknown") .. ", Lua sandbox " .. ((admin_lua_sandbox(ctx) or {}).profile or "strict")),
      ui.text("Doors loaded: " .. tostring(#(admin_doors(ctx) or {})) .. " (" .. tostring(private) .. " private, " .. tostring(hidden) .. " hidden)"),
      ui.text("Connected sessions: " .. tostring(#(ctx.presence.all_users or {}))),
      ui.text("Known users: " .. tostring(#(admin_users(ctx) or {}))),
      ui.text("Database: " .. ((admin_storage(ctx) and admin_storage(ctx).database_path) or admin_database_path(ctx) or "unknown")),
      ui.text("State records: " .. tostring((counts.user or 0) + (counts.room or 0) + (counts.global or 0))),
    }),
    admin_panel("Quick Actions", {
      ui.nav_button("home-doors", "Doors", "doors"),
      ui.nav_button("home-users", "Users", "users"),
      ui.nav_button("home-access", "Access Control", "access"),
      ui.nav_button("home-storage", "Storage", "storage"),
      ui.nav_button("home-runtime", "Runtime", "runtime"),
      ui.nav_button("home-logs", "Logs", "logs"),
      ui.nav_button("home-maintenance", "Maintenance", "maintenance"),
    }),
  })
end

local function door_row(ctx, door)
  local roles = door.roles or door_role_policy(ctx)[door.id or ""]
  local role_text = "roles all"
  if roles and #roles > 0 then
    role_text = "roles " .. join_roles(roles)
  end
  local disabled = disabled_doors(ctx)
  local door_id = door.id or "unknown"
  local enabled = not disabled[door_id]
  local label = door_id .. " - " .. (door.name or door_id)
  local detail = "runtime " .. (door.runtime or "lua") .. ", visibility " .. (door.visibility or "public") .. ", access " .. (door.access or "public") .. ", " .. role_text
  local nodes = {
    ui.text(label),
    ui.text("  " .. detail),
    ui.nav_button("door-detail-" .. door_id, "Details", "doors/detail:" .. door_id),
  }
  if door_id == "admin" then
    table.insert(nodes, ui.text("  admin door is always enabled"))
  else
    table.insert(nodes, ui.checkbox("door-enabled-" .. door_id, "Enabled", enabled, "toggle_door_enabled"))
  end
  return admin_panel(label, nodes)
end

local function render_doors(ctx)
  local enabled = sorted_doors(ctx, true)
  local all_doors = sorted_doors(ctx, false)
  local selected = selected_nav_door(ctx)
  local order_items = {}
  if #enabled == 0 then
    order_items = nil
  else
    for index, door in ipairs(enabled) do
      local door_id = tostring(door.id or "unknown")
      local marker = ""
      if selected == door_id then
        marker = " [selected]"
      end
      table.insert(order_items, ui.item(string.format("%02d. %s (%s)%s", index, door.name or door_id, door_id, marker), "select_door_order:" .. door_id))
    end
  end
  local panels = {
    admin_panel("Navigation Menu Order", {
      ui.text("Enabled doors appear here in the trusted navigation order."),
      ui.text("Use left/right to choose a door, press enter to select it, then use `=` or `-` while the main content panel is focused to move it."),
      order_items and ui.dynamic_list("door-order-list", order_items) or ui.text("No enabled doors are available to reorder."),
      ui.text("Selected for reordering: " .. (selected ~= "" and selected or "none")),
      ui.text("Use the dynamic list to select a door, then `=` / `-` to move it."),
    }),
    admin_panel("Manifest Controls", {
      ui.text("Loaded manifests are read from: " .. (admin_doors_dir(ctx) or "unknown")),
      ui.button("reload-manifests", "Reload Manifests", "reload_manifests"),
    }),
  }
  for _, door in ipairs(all_doors) do
    table.insert(panels, door_row(ctx, door))
  end
  if #all_doors == 0 then
    table.insert(panels, admin_panel("Doors", { ui.text("No manifests are loaded.") }))
  end
  local screen = page_shell(ctx, "DOORS", panels)
  screen.capture_keys = true
  return screen
end

local function render_door_detail(ctx, door_id)
  local door = door_by_id(ctx, door_id)
  if not door then
    return page_shell(ctx, "DOOR DETAIL", {
      admin_panel("Missing Door", {
        ui.back_button("back-to-doors", "Back"),
        ui.text("No loaded manifest for door: " .. tostring(door_id)),
      }),
    })
  end
  local roles = door.roles or door_role_policy(ctx)[door.id or ""]
  local setting_count = #(door.settings or {})
  return page_shell(ctx, "DOOR DETAIL", {
    admin_panel("Manifest", {
      ui.back_button("back-to-doors", "Back"),
      ui.text("Door ID: " .. (door.id or "unknown")),
      ui.text("Name: " .. (door.name or "unknown")),
      ui.text("Runtime: " .. (door.runtime or "lua")),
      ui.text("Entry: " .. (door.entry or "none")),
      ui.text("Command: " .. command_text(door.command)),
      ui.text("Visibility: " .. (door.visibility or "public")),
      ui.text("Access: " .. (door.access or "public")),
      ui.text("Allowlist entries: " .. tostring(door.allowlist_count or 0)),
      ui.text("Lua sandbox profile: " .. (door.sandbox_profile or "strict")),
      ui.text("Role visibility: " .. (roles and #roles > 0 and join_roles(roles) or "all roles")),
      ui.text("Operator settings: " .. tostring(setting_count)),
      setting_count > 0 and ui.nav_button("door-settings-" .. (door.id or ""), "Edit Door Settings", "settings/detail:" .. (door.id or "")) or ui.text("This door does not declare editable operator settings."),
      ui.button("open-door-" .. (door.id or ""), "Open Door As Admin", "open_door_as_admin"),
    }),
  })
end

local function render_settings(ctx)
  local panels = {
    admin_panel("Door Settings", {
      ui.text("Manifests declare setting schemas and defaults; edited station values are stored in node-owned SQLite policy state."),
      ui.text("Doors read the resolved typed values through ctx.settings."),
    }),
  }
  local count = 0
  for _, door in ipairs(sorted_doors(ctx, false)) do
    local settings = door.settings or {}
    if #settings > 0 then
      count = count + 1
      table.insert(panels, admin_panel((door.name or door.id or "Door") .. " (" .. (door.id or "unknown") .. ")", {
        ui.text("Editable settings: " .. tostring(#settings)),
        ui.nav_button("settings-detail-" .. (door.id or "unknown"), "Edit Settings", "settings/detail:" .. (door.id or "")),
      }))
    end
  end
  if count == 0 then
    table.insert(panels, admin_panel("No Settings", {
      ui.text("No loaded door manifests declare a [settings.*] schema yet."),
    }))
  end
  return page_shell(ctx, "DOOR SETTINGS", panels)
end

local function setting_editor_nodes(door, setting)
  local door_id = tostring(door.id or "unknown")
  local name = tostring(setting.name or "unknown")
  local input_id = setting_input_id(door_id, name)
  local kind = tostring(setting.type or "string")
  local label = setting_label(setting)
  local nodes = {
    ui.text("Key: " .. name),
    ui.text("Type: " .. kind),
    ui.text("Default: " .. tostring(setting.default)),
    ui.text("Current: " .. setting_value_text(setting)),
  }
  local choices = setting_choices_text(setting)
  if choices ~= "" then
    table.insert(nodes, ui.text("Choices: " .. choices))
  end
  if kind == "bool" then
    table.insert(nodes, ui.checkbox(input_id, label, setting_value(setting) == true, "save_door_setting_bool:" .. door_id .. ":" .. name))
  elseif kind == "textarea" or kind == "markdown" then
    table.insert(nodes, ui.textarea(input_id, label, setting_value_text(setting)))
    table.insert(nodes, ui.button("save-" .. input_id, "Save " .. label, "save_door_setting:" .. door_id .. ":" .. name))
  else
    table.insert(nodes, ui.input(input_id, label, setting_value_text(setting)))
    table.insert(nodes, ui.button("save-" .. input_id, "Save " .. label, "save_door_setting:" .. door_id .. ":" .. name))
  end
  table.insert(nodes, ui.button("reset-" .. input_id, "Reset To Manifest Default", "reset_door_setting:" .. door_id .. ":" .. name))
  return nodes
end

local function render_settings_detail(ctx, door_id)
  local door = door_by_id(ctx, door_id)
  if not door then
    return page_shell(ctx, "DOOR SETTINGS", {
      admin_panel("Missing Door", {
        ui.back_button("back-to-settings", "Back"),
        ui.text("No loaded manifest for door: " .. tostring(door_id)),
      }),
    })
  end
  local panels = {
    admin_panel("Door", {
      ui.back_button("back-to-settings", "Back"),
      ui.text("Door ID: " .. (door.id or "unknown")),
      ui.text("Name: " .. (door.name or "unknown")),
      ui.text("Settings are applied without editing manifest.toml or restarting the daemon."),
    }),
  }
  for _, setting in ipairs(door.settings or {}) do
    table.insert(panels, admin_panel(setting_label(setting), setting_editor_nodes(door, setting)))
  end
  if #(door.settings or {}) == 0 then
    table.insert(panels, admin_panel("No Settings", {
      ui.text("This door does not declare editable operator settings."),
    }))
  end
  return page_shell(ctx, "DOOR SETTINGS / " .. tostring(door_id), panels)
end

local function render_users(ctx)
  local panels = {}
  for _, user in ipairs(admin_users(ctx) or {}) do
    local online = "offline"
    if user.online then
      online = "online"
    end
    table.insert(panels, admin_panel(display_name(user), {
      ui.text("Display name: " .. display_name(user)),
      ui.text("Role: " .. (user.role or "member") .. " / " .. online),
      ui.text("Fingerprint: " .. (user.fingerprint or "unknown")),
      ui.text("First seen: " .. (user.first_seen or "unknown")),
      ui.text("Last seen: " .. (user.last_seen or "unknown")),
      ui.nav_button("user-detail-" .. (user.fingerprint or "unknown"), "Details", "users/detail:" .. (user.fingerprint or "unknown")),
    }))
  end
  if #panels == 0 then
    panels = { admin_panel("Known Users", { ui.text("No users have authenticated yet.") }) }
  end
  return page_shell(ctx, "USERS", panels)
end

local function render_user_detail(ctx, fingerprint)
  local user = user_by_fingerprint(ctx, fingerprint)
  if not user then
    return page_shell(ctx, "USER DETAIL", {
      admin_panel("Missing User", {
        ui.back_button("back-to-users", "Back"),
        ui.text("No known user with fingerprint: " .. tostring(fingerprint)),
      }),
    })
  end
  return page_shell(ctx, "USER DETAIL", {
    admin_panel("Identity", {
      ui.back_button("back-to-users", "Back"),
      ui.text("Display name: " .. display_name(user)),
      ui.text("Fingerprint: " .. (user.fingerprint or "unknown")),
      ui.text("Public key: " .. (user.public_key or "unknown")),
      ui.text("Role: " .. (user.role or "member")),
      ui.text("Status line: " .. (trim(user.status_line or "") ~= "" and user.status_line or "none")),
      ui.text("Bio: " .. (trim(user.bio or "") ~= "" and user.bio or "none")),
      ui.text("First seen: " .. (user.first_seen or "unknown")),
      ui.text("Last seen: " .. (user.last_seen or "unknown")),
      ui.text("Online: " .. tostring(user.online or false)),
    }),
    admin_panel("Access Shortcuts", {
      ui.text("Use the Access Control page to edit role policy or allowlists."),
      ui.nav_button("user-access", "Access Control", "access"),
    }),
  })
end

local function allowlist_lines(values)
  local nodes = {}
  for _, value in ipairs(values or {}) do
    table.insert(nodes, ui.text("- " .. short_key(value)))
  end
  if #nodes == 0 then
    return { ui.text("No entries.") }
  end
  return nodes
end

local function door_allowlist_lines(ctx)
  local nodes = {}
  for _, door in ipairs(admin_doors(ctx) or {}) do
    if (door.allowlist_count or 0) > 0 then
      table.insert(nodes, ui.text("- " .. (door.id or "unknown") .. ": " .. tostring(door.allowlist_count) .. " entries"))
    end
  end
  if #nodes == 0 then
    return { ui.text("No door-specific allowlists reported by loaded manifests.") }
  end
  return nodes
end

local function sorted_map_keys(values)
  local keys = {}
  for key, _ in pairs(values or {}) do
    table.insert(keys, tostring(key))
  end
  table.sort(keys)
  return keys
end

local function moderation_entry_lines(values)
  local nodes = {}
  for _, public_key in ipairs(sorted_map_keys(values)) do
    local entry = values[public_key] or {}
    local reason = trim(entry.reason or "")
    if reason == "" then
      reason = "no reason recorded"
    end
    local expires = trim(entry.expires_at or "")
    if expires ~= "" then
      expires = " until " .. expires
    end
    table.insert(nodes, ui.text("- " .. short_key(public_key) .. ": " .. reason .. expires))
  end
  if #nodes == 0 then
    return { ui.text("No keys.") }
  end
  return nodes
end

local function rate_limit_lines(ctx)
  local nodes = {}
  for _, public_key in ipairs(sorted_map_keys(user_rate_limits(ctx))) do
    local limit = user_rate_limits(ctx)[public_key] or {}
    table.insert(nodes, ui.text("- " .. short_key(public_key) .. ": events/min " .. tostring(limit.events_per_minute or 0) .. ", opens/min " .. tostring(limit.opens_per_minute or 0)))
  end
  if #nodes == 0 then
    return { ui.text("No per-user rate overrides.") }
  end
  return nodes
end

local function moderation_note_lines(ctx)
  local notes = moderation_notes(ctx)
  local nodes = {}
  local start = math.max(1, #notes - 7)
  for index = start, #notes do
    local note = notes[index] or {}
    table.insert(nodes, ui.text("- " .. short_key(note.public_key or "") .. ": " .. tostring(note.message or "")))
  end
  if #nodes == 0 then
    return { ui.text("No moderation notes.") }
  end
  return nodes
end

local function render_moderation(ctx)
  return page_shell(ctx, "MODERATION", {
    admin_panel("Survival Kit", {
      ui.text("Banned keys: " .. tostring(count_map(banned_keys(ctx)))),
      ui.text("Muted keys: " .. tostring(count_map(muted_keys(ctx)))),
      ui.text("Rate-limited keys: " .. tostring(count_map(user_rate_limits(ctx)))),
      ui.text("Recent station activity is on the Logs page."),
    }),
    admin_panel("Key Controls", {
      ui.input("moderation-public-key", "passport public key", ""),
      ui.input("moderation-reason", "reason or note", ""),
      ui.input("moderation-events-per-minute", "events per minute, blank to leave 0", ""),
      ui.input("moderation-opens-per-minute", "door opens per minute, blank to leave 0", ""),
      ui.button("ban-key", "Ban Key", "ban_key"),
      ui.button("unban-key", "Unban Key", "unban_key"),
      ui.button("mute-key", "Mute Key", "mute_key"),
      ui.button("unmute-key", "Unmute Key", "unmute_key"),
      ui.button("set-user-rate-limit", "Set Rate Limit", "set_user_rate_limit"),
      ui.button("clear-user-rate-limit", "Clear Rate Limit", "clear_user_rate_limit"),
      ui.button("record-moderation-note", "Record Note", "record_moderation_note"),
    }),
    admin_panel("Banned Keys", moderation_entry_lines(banned_keys(ctx))),
    admin_panel("Muted Keys", moderation_entry_lines(muted_keys(ctx))),
    admin_panel("Rate Limits", rate_limit_lines(ctx)),
    admin_panel("Recent Notes", moderation_note_lines(ctx)),
  })
end

local function render_access(ctx)
  local hidden, private = hidden_private_counts(ctx)
  return page_shell(ctx, "ACCESS CONTROL", {
    admin_panel("Station Access", {
      ui.text("Mode: " .. (ctx.node.access_mode or "public")),
      ui.text("Station allowlist entries: " .. tostring(#(admin_station_allowlist(ctx) or {}))),
      ui.text("Admin entries: " .. tostring(#(admin_admins(ctx) or {}))),
      ui.text("Hidden doors: " .. tostring(hidden)),
      ui.text("Private doors: " .. tostring(private)),
    }),
    admin_panel("Station Allowlist", allowlist_lines(admin_station_allowlist(ctx) or {})),
    admin_panel("Admins", allowlist_lines(admin_admins(ctx) or {})),
    admin_panel("Station Roles", role_lines(ctx)),
    admin_panel("Admission Policy", {
      ui.text("Station entry is controlled by the station allowlist and configured admins."),
      ui.text("Assigned roles change permissions after admission; they do not bypass invite-only entry."),
    }),
    admin_panel("Assign Role", {
      ui.input("role-public-key", "passport public key", ""),
      ui.input("role-name", "member, moderator, admin, sysop, or custom role", ""),
      ui.button("assign-role", "Assign Role", "assign_role"),
      ui.button("remove-role", "Remove Role", "remove_role"),
    }),
    admin_panel("Door Role Visibility", {
      ui.input("door-id", "door id", ""),
      ui.input("door-roles", "comma separated roles that can see/open the door", ""),
      ui.button("set-door-roles", "Set Door Roles", "set_door_roles"),
      ui.button("clear-door-roles", "Clear Door Roles", "clear_door_roles"),
    }),
    admin_panel("Door Allowlists", door_allowlist_lines(ctx)),
  })
end

local function largest_state_lines(ctx)
  local records = {}
  for _, record in ipairs((admin_storage(ctx) and admin_storage(ctx).state_records) or {}) do
    table.insert(records, record)
  end
  table.sort(records, function(a, b)
    return (tonumber(a.bytes or 0) or 0) > (tonumber(b.bytes or 0) or 0)
  end)
  local nodes = {}
  for index, record in ipairs(records) do
    if index > 8 then
      break
    end
    table.insert(nodes, ui.text("- " .. (record.door_id or "unknown") .. " / " .. (record.scope or "?") .. " / " .. short_key(record.scope_id) .. ": " .. tostring(record.bytes or 0) .. " bytes, " .. (record.updated_at or "unknown")))
  end
  if #nodes == 0 then
    return { ui.text("No scoped door state has been written yet.") }
  end
  return nodes
end

local function storage_door_buttons(ctx)
  local seen = {}
  local nodes = {}
  for _, record in ipairs((admin_storage(ctx) and admin_storage(ctx).state_records) or {}) do
    local door_id = record.door_id or "unknown"
    if not seen[door_id] then
      seen[door_id] = true
      table.insert(nodes, ui.nav_button("storage-door-" .. door_id, door_id, "storage/door:" .. door_id))
    end
  end
  if #nodes == 0 then
    return { ui.text("No door state records to inspect.") }
  end
  return nodes
end

local function render_storage(ctx)
  local counts, bytes, doors = state_counts(ctx)
  local panels = {
    admin_panel("Database", {
      ui.text("Path: " .. ((admin_storage(ctx) and admin_storage(ctx).database_path) or admin_database_path(ctx) or "unknown")),
      ui.text("Doors with state: " .. tostring(count_map(doors))),
      ui.text("User records: " .. tostring(counts.user or 0) .. " / " .. tostring(bytes.user or 0) .. " bytes"),
      ui.text("Room records: " .. tostring(counts.room or 0) .. " / " .. tostring(bytes.room or 0) .. " bytes"),
      ui.text("Global records: " .. tostring(counts.global or 0) .. " / " .. tostring(bytes.global or 0) .. " bytes"),
    }),
    admin_panel("State By Door", storage_door_buttons(ctx)),
    admin_panel("Largest State Blobs", largest_state_lines(ctx)),
  }
  if setting(ctx, "show_storage_maintenance_actions", true) then
    table.insert(panels, admin_panel("Maintenance Actions", {
      ui.text("Clearing arbitrary door state requires a node-owned admin storage effect and confirmation view."),
      ui.button("clear-door-user-state", "Clear User State For Door", "storage_clear_not_ready"),
      ui.button("clear-door-room-state", "Clear Room State For Door", "storage_clear_not_ready"),
      ui.button("clear-door-global-state", "Clear Global State For Door", "storage_clear_not_ready"),
    }))
  end
  return page_shell(ctx, "STORAGE", panels)
end

local function render_storage_door(ctx, door_id)
  local nodes = { ui.back_button("back-to-storage", "Back") }
  for _, record in ipairs((admin_storage(ctx) and admin_storage(ctx).state_records) or {}) do
    if record.door_id == door_id then
      table.insert(nodes, ui.text("- " .. (record.scope or "?") .. " / " .. short_key(record.scope_id) .. ": " .. tostring(record.bytes or 0) .. " bytes, " .. (record.updated_at or "unknown")))
      table.insert(nodes, ui.nav_button("storage-scope-" .. (record.scope or "?"), "Scope " .. (record.scope or "?"), "storage/door:" .. door_id .. "/scope:" .. (record.scope or "?")))
    end
  end
  if #nodes == 1 then
    table.insert(nodes, ui.text("No state records for " .. tostring(door_id)))
  end
  return page_shell(ctx, "STORAGE / " .. tostring(door_id), { admin_panel("State Records", nodes) })
end

local function render_storage_scope(ctx, door_id, scope)
  local nodes = { ui.back_button("back-to-storage-door", "Back") }
  for _, record in ipairs((admin_storage(ctx) and admin_storage(ctx).state_records) or {}) do
    if record.door_id == door_id and record.scope == scope then
      table.insert(nodes, ui.text("- " .. short_key(record.scope_id) .. ": " .. tostring(record.bytes or 0) .. " bytes, " .. (record.updated_at or "unknown")))
    end
  end
  if #nodes == 1 then
    table.insert(nodes, ui.text("No " .. tostring(scope) .. " records for " .. tostring(door_id)))
  end
  return page_shell(ctx, "STORAGE / " .. tostring(door_id) .. " / " .. tostring(scope), { admin_panel("Scope Records", nodes) })
end

local function render_runtime(ctx)
  local lua = admin_lua_sandbox(ctx) or {}
  local runtime_nodes = {
    ui.text("Default runtime: " .. (admin_default_runtime(ctx) or "unknown")),
    ui.text("Lua sandbox profile: " .. (lua.profile or "strict")),
    ui.text("Lua max memory KB: " .. tostring(lua.max_memory_kb or "unknown")),
    ui.text("Lua max execution ms: " .. tostring(lua.max_execution_ms or "unknown")),
    ui.text("Stdio runtime: executes manifest command with canonical JSON over stdin/stdout"),
  }
  local door_nodes = {}
  for _, door in ipairs(admin_doors(ctx) or {}) do
    table.insert(door_nodes, ui.text("- " .. (door.id or "unknown") .. ": " .. (door.runtime or "lua") .. " (" .. ((door.entry and door.entry ~= "" and door.entry) or command_text(door.command)) .. ")"))
  end
  if #door_nodes == 0 then
    door_nodes = { ui.text("No doors loaded.") }
  end
  local panels = {
    admin_panel("Runtime Summary", runtime_nodes),
    admin_panel("Door Runtime List", door_nodes),
  }
  if setting(ctx, "show_runtime_actions", true) then
    table.insert(panels, admin_panel("Actions", {
      ui.button("runtime-health-check", "Run Door Health Check", "health_check"),
      ui.button("runtime-show-sandbox", "Show Sandbox Config", "show_sandbox_config"),
      ui.button("runtime-python-sdk", "Show Stdio/Python Status", "show_python_sdk_status"),
    }))
  end
  return page_shell(ctx, "RUNTIME", panels)
end

local function render_logs(ctx)
  return page_shell(ctx, "LOGS", {
    admin_panel("Recent Station Events", event_lines(ctx)),
    admin_panel("Recent Admin Notices", notice_lines(ctx)),
    admin_panel("Filters", {
      ui.text("Event filtering is next-layer work; this is the in-memory ring buffer."),
      ui.button("clear-events", "Clear In-Memory Event Log", "clear_events"),
      ui.button("clear-notices", "Clear Admin Notices", "clear_notices"),
    }),
  })
end

local function render_maintenance(ctx)
  return page_shell(ctx, "MAINTENANCE", {
    admin_panel("Station Maintenance", {
      ui.text("Maintenance mode: " .. maintenance_mode(ctx)),
      ui.text("Recorded checkpoints: " .. tostring(maintenance_count(ctx))),
      ui.text("Config: station " .. (ctx.node.name or "unknown") .. ", doors " .. (admin_doors_dir(ctx) or "unknown")),
      ui.text("Build/version: development build"),
    }),
    admin_panel("Station Notice", {
      ui.input("station-notice", "message to notify all connected users", draft_notice(ctx)),
      ui.button("send-notice", "Send Station Notice", "send_notice"),
    }),
    admin_panel("Actions", {
      ui.button("health-check", "Run Health Check", "health_check"),
      ui.button("reload-manifests", "Reload Door Manifests", "reload_manifests"),
      ui.button("checkpoint", "Record Maintenance Checkpoint", "checkpoint"),
      ui.button("enable-maintenance", "Enable Maintenance Mode", "enable_maintenance"),
      ui.button("disable-maintenance", "Disable Maintenance Mode", "disable_maintenance"),
      ui.nav_button("confirm-reset-maintenance", "Reset Maintenance State", "confirm:reset_maintenance"),
    }),
  })
end

local function render_confirm(ctx, action)
  local label = tostring(action or "")
  local copy = "Confirm action: " .. label
  if label == "reset_maintenance" then
    copy = "Reset maintenance counters, mode, and admin notices."
  end
  return page_shell(ctx, "CONFIRM", {
    admin_panel("Confirmation", {
      ui.text(copy),
      ui.button("confirm-action", "Confirm", "confirm:" .. label),
      ui.back_button("cancel-confirm", "Cancel"),
    }),
  })
end

function view(ctx)
  local page = current_page(ctx)
  if page == "doors" then
    return render_doors(ctx)
  elseif has_prefix(page, "doors/detail:") then
    return render_door_detail(ctx, strip_prefix(page, "doors/detail:"))
  elseif page == "settings" then
    return render_settings(ctx)
  elseif has_prefix(page, "settings/detail:") then
    return render_settings_detail(ctx, strip_prefix(page, "settings/detail:"))
  elseif page == "users" then
    return render_users(ctx)
  elseif has_prefix(page, "users/detail:") then
    return render_user_detail(ctx, strip_prefix(page, "users/detail:"))
  elseif page == "access" then
    return render_access(ctx)
  elseif page == "moderation" then
    return render_moderation(ctx)
  elseif page == "storage" then
    return render_storage(ctx)
  elseif has_prefix(page, "storage/door:") and page:find("/scope:", 1, true) then
    local door_id, scope = page:match("^storage/door:(.-)/scope:(.+)$")
    return render_storage_scope(ctx, door_id, scope)
  elseif has_prefix(page, "storage/door:") then
    return render_storage_door(ctx, strip_prefix(page, "storage/door:"))
  elseif page == "runtime" then
    return render_runtime(ctx)
  elseif page == "logs" then
    return render_logs(ctx)
  elseif page == "maintenance" then
    return render_maintenance(ctx)
  elseif has_prefix(page, "confirm:") then
    return render_confirm(ctx, strip_prefix(page, "confirm:"))
  end
  return render_home(ctx)
end

function update(ctx, event)
  local action = event and event.action or ""
  local page = current_page(ctx)
  if action == "confirm:reset_maintenance" then
    admin_op(ctx, { op = "reset_maintenance" })
    ctx.nav:reset("maintenance")
    ctx.effects.notify("Maintenance state reset.", "warning", "self")
  elseif ctx.nav:handle(event, "home") then
    return view(ctx)
  elseif event and event.kind == "submit" and event.target == "station-notice" then
    local message = event.values and event.values["station-notice"] or ""
    if message ~= "" then
      append_notice(ctx, "Station notice: " .. message)
      ctx.effects.set_state("user", "station_notice", "")
      ctx.effects.notify("Station notice: " .. message, "warning", "all")
    end
  elseif action == "send_notice" then
    local message = draft_notice(ctx)
    if event.values and event.values["station-notice"] and event.values["station-notice"] ~= "" then
      message = event.values["station-notice"]
    end
    if message ~= "" then
      append_notice(ctx, "Station notice: " .. message)
      ctx.effects.set_state("user", "station_notice", "")
      ctx.effects.notify("Station notice: " .. message, "warning", "all")
    else
      ctx.effects.notify("Enter a station notice first.", "warning", "self")
    end
  elseif action == "health_check" then
    append_notice(ctx, "Health check requested by " .. (ctx.user.fingerprint or "unknown"))
    ctx.effects.notify("Station health check recorded.", "info", "self")
  elseif action == "checkpoint" then
    local count = maintenance_count(ctx) + 1
    append_notice(ctx, "Maintenance checkpoint " .. tostring(count))
    admin_op(ctx, { op = "record_maintenance_checkpoint" })
    ctx.effects.notify("Maintenance checkpoint recorded.", "info", "self")
  elseif action == "enable_maintenance" then
    admin_op(ctx, { op = "set_maintenance", maintenance = true })
    append_notice(ctx, "Maintenance mode enabled by " .. (ctx.user.fingerprint or "unknown"))
    ctx.effects.notify("Station maintenance mode is now enabled.", "warning", "all")
  elseif action == "disable_maintenance" then
    admin_op(ctx, { op = "set_maintenance", maintenance = false })
    append_notice(ctx, "Maintenance mode disabled by " .. (ctx.user.fingerprint or "unknown"))
    ctx.effects.notify("Station maintenance mode is now disabled.", "info", "all")
  elseif action == "clear_notices" then
    admin_op(ctx, { op = "clear_station_notices" })
    ctx.effects.notify("Admin events cleared.", "info", "self")
  elseif action == "clear_events" then
    admin_op(ctx, { op = "clear_event_log" })
    ctx.effects.notify("In-memory event log cleared.", "info", "self")
  elseif action == "reset_maintenance" then
    ctx.nav:push("confirm:reset_maintenance")
  elseif action == "assign_role" then
    local public_key = trim(event_value(event, "role-public-key", ""))
    local role = trim(event_value(event, "role-name", "")):lower()
    if public_key == "" or role == "" then
      ctx.effects.notify("Enter a public key and role.", "warning", "self")
    else
      admin_op(ctx, { op = "set_user_role", public_key = public_key, role = role })
      append_notice(ctx, "Assigned role " .. role .. " to " .. public_key)
      ctx.effects.notify("Role assigned. Once admitted, the user gets it on next sign-in.", "info", "self")
    end
  elseif action == "remove_role" then
    local public_key = trim(event_value(event, "role-public-key", ""))
    if public_key == "" then
      ctx.effects.notify("Enter a public key to remove.", "warning", "self")
    else
      admin_op(ctx, { op = "set_user_role", public_key = public_key, role = "" })
      append_notice(ctx, "Removed station role for " .. public_key)
      ctx.effects.notify("Role removed. Existing sessions keep their current role until reconnect.", "warning", "self")
    end
  elseif action == "set_door_roles" then
    local door_id = trim(event_value(event, "door-id", ""))
    local roles = split_roles(event_value(event, "door-roles", ""))
    if door_id == "" or #roles == 0 then
      ctx.effects.notify("Enter a door id and at least one role.", "warning", "self")
    else
      admin_op(ctx, { op = "set_door_roles", door_id = door_id, roles = roles })
      append_notice(ctx, "Door " .. door_id .. " visibility set to roles: " .. join_roles(roles))
      ctx.effects.notify("Door role visibility updated.", "info", "self")
    end
  elseif action == "clear_door_roles" then
    local door_id = trim(event_value(event, "door-id", ""))
    if door_id == "" then
      ctx.effects.notify("Enter a door id to clear.", "warning", "self")
    else
      admin_op(ctx, { op = "set_door_roles", door_id = door_id })
      append_notice(ctx, "Cleared role visibility for door " .. door_id)
      ctx.effects.notify("Door now uses its manifest visibility/access.", "info", "self")
    end
  elseif action == "ban_key" then
    local public_key = trim(event_value(event, "moderation-public-key", ""))
    local reason = trim(event_value(event, "moderation-reason", ""))
    if public_key == "" then
      ctx.effects.notify("Enter a public key to ban.", "warning", "self")
    else
      admin_op(ctx, { op = "ban_key", public_key = public_key, reason = reason })
      ctx.effects.notify("Key banned and active sessions will be disconnected.", "warning", "self")
    end
  elseif action == "unban_key" then
    local public_key = trim(event_value(event, "moderation-public-key", ""))
    if public_key == "" then
      ctx.effects.notify("Enter a public key to unban.", "warning", "self")
    else
      admin_op(ctx, { op = "unban_key", public_key = public_key })
      ctx.effects.notify("Key unbanned.", "info", "self")
    end
  elseif action == "mute_key" then
    local public_key = trim(event_value(event, "moderation-public-key", ""))
    local reason = trim(event_value(event, "moderation-reason", ""))
    if public_key == "" then
      ctx.effects.notify("Enter a public key to mute.", "warning", "self")
    else
      admin_op(ctx, { op = "mute_key", public_key = public_key, reason = reason })
      ctx.effects.notify("Key muted.", "warning", "self")
    end
  elseif action == "unmute_key" then
    local public_key = trim(event_value(event, "moderation-public-key", ""))
    if public_key == "" then
      ctx.effects.notify("Enter a public key to unmute.", "warning", "self")
    else
      admin_op(ctx, { op = "unmute_key", public_key = public_key })
      ctx.effects.notify("Key unmuted.", "info", "self")
    end
  elseif action == "set_user_rate_limit" then
    local public_key = trim(event_value(event, "moderation-public-key", ""))
    local events_per_minute = math.max(0, math.floor(tonumber(event_value(event, "moderation-events-per-minute", "0")) or 0))
    local opens_per_minute = math.max(0, math.floor(tonumber(event_value(event, "moderation-opens-per-minute", "0")) or 0))
    if public_key == "" then
      ctx.effects.notify("Enter a public key to rate-limit.", "warning", "self")
    elseif events_per_minute == 0 and opens_per_minute == 0 then
      ctx.effects.notify("Enter at least one positive limit.", "warning", "self")
    else
      admin_op(ctx, { op = "set_user_rate_limit", public_key = public_key, events_per_minute = events_per_minute, opens_per_minute = opens_per_minute })
      ctx.effects.notify("User rate limit updated.", "info", "self")
    end
  elseif action == "clear_user_rate_limit" then
    local public_key = trim(event_value(event, "moderation-public-key", ""))
    if public_key == "" then
      ctx.effects.notify("Enter a public key to clear rate limits.", "warning", "self")
    else
      admin_op(ctx, { op = "set_user_rate_limit", public_key = public_key, reset = true })
      ctx.effects.notify("User rate limit cleared.", "info", "self")
    end
  elseif action == "record_moderation_note" then
    local public_key = trim(event_value(event, "moderation-public-key", ""))
    local message = trim(event_value(event, "moderation-reason", ""))
    if public_key == "" or message == "" then
      ctx.effects.notify("Enter a public key and note.", "warning", "self")
    else
      admin_op(ctx, { op = "record_moderation_note", public_key = public_key, message = message })
      ctx.effects.notify("Moderation note recorded.", "info", "self")
    end
  elseif action == "toggle_door_enabled" then
    local target = event and event.target or ""
    if not has_prefix(target, "door-enabled-") then
      ctx.effects.notify("Unknown door toggle target.", "warning", "self")
    else
      local door_id = strip_prefix(target, "door-enabled-")
      local checked = event and event.values and event.values.checked == "true"
      if checked then
        admin_op(ctx, { op = "set_door_enabled", door_id = door_id, enabled = true })
        append_notice(ctx, "Enabled door " .. door_id)
        ctx.effects.notify("Door enabled: " .. door_id, "info", "self")
      else
        admin_op(ctx, { op = "set_door_enabled", door_id = door_id, enabled = false })
        append_notice(ctx, "Disabled door " .. door_id)
        ctx.effects.notify("Door disabled: " .. door_id, "warning", "self")
      end
    end
  elseif has_prefix(action, "select_door_order:") then
    local door_id = strip_prefix(action, "select_door_order:")
    ctx.effects.set_state("user", "selected_nav_door", door_id)
    ctx.effects.notify("Selected door for reordering: " .. door_id, "info", "self")
  elseif action == "move_door_order_up" or (page == "doors" and event and event.kind == "key" and (event.key == "=" or event.key == "+")) then
    local door_id = selected_nav_door(ctx)
    local ok, err = move_enabled_door_order(ctx, door_id, -1)
    if ok then
      append_notice(ctx, "Moved door earlier in navigation: " .. door_id)
      ctx.effects.notify("Door moved earlier in navigation order.", "info", "self")
    else
      ctx.effects.notify(err, "warning", "self")
    end
  elseif action == "move_door_order_down" or (page == "doors" and event and event.kind == "key" and event.key == "-") then
    local door_id = selected_nav_door(ctx)
    local ok, err = move_enabled_door_order(ctx, door_id, 1)
    if ok then
      append_notice(ctx, "Moved door later in navigation: " .. door_id)
      ctx.effects.notify("Door moved later in navigation order.", "info", "self")
    else
      ctx.effects.notify(err, "warning", "self")
    end
  elseif has_prefix(action, "save_door_setting_bool:") then
    local door_id, name = setting_action_suffix(action)
    save_door_setting(ctx, door_id, name, event and event.values and event.values.checked or "false")
  elseif has_prefix(action, "save_door_setting:") then
    local door_id, name = setting_action_suffix(action)
    local input_id = setting_input_id(door_id, name)
    save_door_setting(ctx, door_id, name, event_value(event, input_id, ""))
  elseif has_prefix(action, "reset_door_setting:") then
    local door_id, name = setting_action_suffix(action)
    reset_door_setting(ctx, door_id, name)
  elseif action == "reload_manifests" then
    admin_op(ctx, { op = "reload_manifests" })
    append_notice(ctx, "Manifest reload requested by " .. (ctx.user.fingerprint or "unknown"))
    ctx.effects.notify("Manifest reload requested. Newly added doors should appear after the node refreshes its manifest list.", "info", "self")
  elseif action == "open_door_as_admin" then
    ctx.effects.notify("Opening a door from inside another door is not enabled yet; use the trusted door rail.", "warning", "self")
  elseif action == "storage_clear_not_ready" then
    ctx.effects.notify("Cross-door state clearing needs a dedicated node-owned storage effect and confirmation view.", "warning", "self")
  elseif action == "show_sandbox_config" then
    append_notice(ctx, "Lua sandbox config inspected by " .. (ctx.user.fingerprint or "unknown"))
    ctx.effects.notify("Sandbox config is shown on the Runtime page.", "info", "self")
  elseif action == "show_python_sdk_status" then
    append_notice(ctx, "Python SDK status inspected by " .. (ctx.user.fingerprint or "unknown"))
    ctx.effects.notify("Python doors run through the stdio runtime command when python3 is available.", "info", "self")
  end
  return view(ctx)
end
