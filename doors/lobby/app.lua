local ui = phosphornet.ui

local function trim(value)
  return tostring(value or ""):match("^%s*(.-)%s*$")
end

local function station_name(ctx)
  return ctx.node.name or "LOCALBOX"
end

local function setting(ctx, key, fallback)
  if ctx.settings and ctx.settings[key] ~= nil then
    return ctx.settings[key]
  end
  return fallback
end

local function guest_name(fingerprint)
  local value = tostring(fingerprint or "unknown")
  local first = value:match("^[^-]+") or value
  if first == "" then
    first = "unknown"
  end
  return "guest-" .. first
end

local function display_name_for_user(user)
  local name = trim(user and user.display_name or "")
  if name ~= "" then
    return name
  end
  return guest_name(user and user.fingerprint)
end

local function current_display_name(ctx)
  return display_name_for_user(ctx.user)
end

local function visit_count(ctx)
  return tonumber(ctx.store:get("user", "lobby_visits", 0)) or 0
end

local function station_notice(ctx)
  local notices = (ctx.states.global or {}).notices or {}
  if #notices > 0 then
    local latest = tostring(notices[#notices] or "")
    return latest:gsub("^Station notice:%s*", "")
  end
  return setting(ctx, "motd", "Public alpha node. Be nice. Things may break.")
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

local function presence_lines(ctx)
  local users = (ctx.presence and ctx.presence.all_users) or {}
  if #users == 0 then
    return { ui.text("Nobody visible yet.") }
  end

  local lines = {}
  for _, user in ipairs(users) do
    local name = display_name_for_user(user)
    local door = user.active_door or "lobby"
    local role = user.role or "member"
    table.insert(lines, ui.text("- " .. name .. " in " .. door .. " · " .. role))
  end
  return lines
end

local function station_panel(ctx)
  local children = {
    ui.text("A small terminal-native station running PhosphorNet."),
    ui.text("Tagline: " .. tostring(setting(ctx, "theme_tagline", "terminal-native public square"))),
    ui.text("You are: " .. current_display_name(ctx)),
    ui.text("Passport: " .. (ctx.user.fingerprint or "unknown")),
    ui.text("Role: " .. (ctx.user.role or "member")),
    ui.text("Online now: " .. tostring(#((ctx.presence and ctx.presence.all_users) or {}))),
    ui.text("Lobby visits recorded: " .. tostring(visit_count(ctx))),
  }
  if trim(ctx.user.display_name or "") == "" then
    table.insert(children, ui.text("Set a display name to feel like a real person inside the station."))
    table.insert(children, ui.button("open-profile", "Open Profile Door", "open_door:profile"))
  end
  return ui.panel("Station", children, gradient_background("vertical", {
    { at = 0.0, color = "#18122b" },
    { at = 0.55, color = "#2b124c" },
    { at = 1.0, color = "#0f3d3e" },
  }))
end

function view(ctx)
  local children = {
    ui.header(station_name(ctx)),
    station_panel(ctx),
  }
  if setting(ctx, "show_online_users", true) then
    table.insert(children, ui.panel("Who Is Here", presence_lines(ctx), gradient_background("vertical", {
      { at = 0.0, color = "#10243a" },
      { at = 0.5, color = "#14344a" },
      { at = 1.0, color = "#18372f" },
    })))
  end
  table.insert(children, ui.panel("Station Notice", {
    ui.text(station_notice(ctx)),
  }, gradient_background("diagonal", {
    { at = 0.0, color = "#24151f" },
    { at = 0.55, color = "#2f2338" },
    { at = 1.0, color = "#3a2f16" },
  })))
  table.insert(children, ui.status("Client renders. Station thinks. Passport identifies."))
  return ui.screen(children, gradient_background("diagonal", {
    { at = 0.0, color = "#0b1020" },
    { at = 0.55, color = "#101827" },
    { at = 1.0, color = "#132a2e" },
  }))
end

function update(ctx, event)
  local action = event and event.action or ""
  if string.sub(action, 1, 10) == "open_door:" then
    local door_id = string.sub(action, 11)
    if door_id ~= "" then
      ctx.effects.transition("open_door", door_id)
    end
  elseif action == "record_visit" then
    ctx.store:set("user", "lobby_visits", visit_count(ctx) + 1)
  elseif action == "reset_visits" then
    ctx.store:set("user", "lobby_visits", 0)
  end
  return view(ctx)
end

function on_join(ctx)
  ctx.store:set("user", "lobby_visits", visit_count(ctx) + 1)
  ctx.effects.notify(current_display_name(ctx) .. " entered the lobby.", "info", "room")
  return view(ctx)
end

function on_leave(ctx)
  ctx.effects.notify(current_display_name(ctx) .. " left the lobby.", "info", "room")
  return view(ctx)
end
