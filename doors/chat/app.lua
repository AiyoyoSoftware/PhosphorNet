local ui = phosphornet.ui

local HELP_LINES = {
  "commands:",
  "/nickname <name> - set your station display name",
  "/tell <display-name> <message> - send a private notice to an online user",
  "/who - list visible online users",
  "/help - show this help",
}

local MAX_RENDERED_LOG_LINES = 64

local function gradient_background(direction, stops)
  return {
    background = {
      kind = "gradient",
      direction = direction,
      stops = stops,
    },
  }
end

local ROOM_PANEL_STYLE = gradient_background("diagonal", {
  { at = 0.0, color = "#101827" },
  { at = 0.55, color = "#17324a" },
  { at = 1.0, color = "#173a32" },
})

local CHAT_LOG_STYLE = gradient_background("vertical", {
  { at = 0.0, color = "#0b1020" },
  { at = 0.5, color = "#151f3a" },
  { at = 1.0, color = "#102f2f" },
})

local function trim(value)
  return tostring(value or ""):match("^%s*(.-)%s*$")
end

local function setting(ctx, key, fallback)
  if ctx.settings and ctx.settings[key] ~= nil then
    return ctx.settings[key]
  end
  return fallback
end

local function int_setting(ctx, key, fallback, minimum)
  local value = tonumber(setting(ctx, key, fallback)) or fallback
  minimum = minimum or 1
  if value < minimum then
    return minimum
  end
  return math.floor(value)
end

local function room_messages(ctx)
  local value = ctx.store:get("room", "messages", {})
  if type(value) ~= "table" then
    return {}
  end
  return value
end

local function guest_name(fingerprint)
  local value = tostring(fingerprint or "unknown")
  local first = value:match("^[^-]+") or value
  if first == "" then
    first = "unknown"
  end
  return "guest-" .. first
end

local function display_name_for_user(user, fallback_fingerprint)
  local name = trim(user and user.display_name or "")
  if name ~= "" then
    return name
  end
  return guest_name((user and user.fingerprint) or fallback_fingerprint)
end

local function display_name(ctx)
  return display_name_for_user(ctx.user, ctx.user and ctx.user.fingerprint)
end

local function fingerprint(ctx)
  return tostring(ctx.user and ctx.user.fingerprint or "unknown")
end

local function public_key(ctx)
  return tostring(ctx.user and ctx.user.public_key or "")
end

local function room_users(ctx)
  return (ctx.presence and ctx.presence.room_users) or {}
end

local function all_users(ctx)
  return (ctx.presence and ctx.presence.all_users) or room_users(ctx)
end

local function profile_for_public_key(ctx, key)
  if key == nil or key == "" then
    return nil
  end
  for _, user in ipairs(room_users(ctx)) do
    if user.public_key == key then
      return user
    end
  end
  for _, user in ipairs(all_users(ctx)) do
    if user.public_key == key then
      return user
    end
  end
  for _, user in ipairs(ctx.users or {}) do
    if user.public_key == key then
      return user
    end
  end
  return nil
end

local function presence_lines(ctx)
  if not setting(ctx, "show_presence", true) then
    return "hidden by station settings"
  end
  local users = room_users(ctx)
  if #users == 0 then
    return "no operators"
  end
  local names = {}
  for _, user in ipairs(users) do
    table.insert(names, display_name_for_user(user, fingerprint(ctx)))
  end
  return table.concat(names, ", ")
end

local function message_sender(ctx, message)
  local key = tostring(message.from_public_key or "")
  if key ~= "" then
    local profile = profile_for_public_key(ctx, key)
    if profile then
      return display_name_for_user(profile, fingerprint(ctx))
    end
  end
  local fallback = trim(message.from_display_name or message.from or "")
  if fallback ~= "" then
    return fallback
  end
  return "unknown"
end

local function message_label(ctx, message)
  local kind = tostring(message.kind or "message")
  local text = tostring(message.text or "")
  if kind == "event" or message.from == "system" then
    return "* " .. text
  end
  return "[" .. message_sender(ctx, message) .. "] " .. text
end

local function message_lines(ctx, notices)
  local lines = {}
  for _, message in ipairs(room_messages(ctx)) do
    table.insert(lines, ui.text(message_label(ctx, message)))
  end
  for _, notice in ipairs(notices or {}) do
    table.insert(lines, ui.text("-!- " .. notice))
  end
  if #lines == 0 then
    return { ui.text("--- no backlog yet ---") }
  end
  if #lines > MAX_RENDERED_LOG_LINES then
    local hidden = #lines - (MAX_RENDERED_LOG_LINES - 1)
    local trimmed = { ui.text("--- " .. tostring(hidden) .. " older entries hidden ---") }
    for index = #lines - (MAX_RENDERED_LOG_LINES - 2), #lines do
      table.insert(trimmed, lines[index])
    end
    lines = trimmed
  end
  return lines
end

local function draft_value(ctx)
  return tostring(ctx.store:get("user", "draft", ""))
end

local function broadcast_room_changed(ctx)
  ctx.effects.broadcast({ kind = "action", target = "chat", action = "room_changed" }, "room")
end

local function append_message(ctx, message)
  local messages = room_messages(ctx)
  table.insert(messages, message)
  local limit = int_setting(ctx, "max_messages", 250, 1)
  while #messages > limit do
    table.remove(messages, 1)
  end
  ctx.store:set("room", "messages", messages)
  broadcast_room_changed(ctx)
end

local function append_notice(notices, notice)
  table.insert(notices, notice)
end

local function set_display_name(ctx, notices, name)
  name = trim(name)
  if name == "" then
    append_notice(notices, "usage: /nickname <name>")
    return
  end
  ctx.effects.update_profile(name)
  broadcast_room_changed(ctx)
  append_notice(notices, "station display name set to " .. name)
end

local function find_user_by_display_name(ctx, name)
  local wanted = string.lower(trim(name))
  if wanted == "" then
    return nil
  end
  for _, user in ipairs(room_users(ctx)) do
    if string.lower(display_name_for_user(user, fingerprint(ctx))) == wanted then
      return user
    end
  end
  return nil
end

local function tell(ctx, notices, payload)
  local target_name, message = trim(payload):match("^(%S+)%s+(.+)$")
  message = trim(message or "")
  if not target_name or message == "" then
    append_notice(notices, "usage: /tell <display-name> <message>")
    return
  end
  local target = find_user_by_display_name(ctx, target_name)
  if not target then
    append_notice(notices, "no online user named " .. target_name)
    return
  end
  local resolved_name = display_name_for_user(target, fingerprint(ctx))
  ctx.effects.notify("[tell from " .. display_name(ctx) .. "] " .. message, "info", "user", target.public_key or "")
  append_notice(notices, "-> " .. resolved_name .. ": " .. message)
end

local function who(ctx, notices)
  local users = all_users(ctx)
  if #users == 0 then
    append_notice(notices, "Online: nobody visible")
    return
  end
  local names = {}
  for _, user in ipairs(users) do
    table.insert(names, display_name_for_user(user, fingerprint(ctx)))
  end
  append_notice(notices, "Online: " .. table.concat(names, ", "))
end

local function handle_command(ctx, notices, text)
  local command, payload = text:match("^(%S+)%s*(.*)$")
  command = string.lower(command or "")
  payload = payload or ""
  if command == "/nickname" then
    set_display_name(ctx, notices, payload)
  elseif command == "/tell" then
    tell(ctx, notices, payload)
  elseif command == "/who" then
    who(ctx, notices)
  elseif command == "/help" then
    for _, line in ipairs(HELP_LINES) do
      append_notice(notices, line)
    end
  else
    append_notice(notices, "unknown command " .. command .. "; try /help")
  end
end

local function chat_screen(ctx, notices)
  local title = tostring(setting(ctx, "room_title", "#station"))
  local topic = tostring(setting(ctx, "topic", "phosphornet room"))
  local screen = ui.screen({
    ui.header(title),
    ui.panel("Room", {
      ui.text("topic: " .. topic .. " " .. tostring(ctx.room and ctx.room.id or "unknown")),
      ui.text("you are: " .. display_name(ctx) .. " passport " .. fingerprint(ctx)),
      ui.text("users: " .. presence_lines(ctx)),
    }, ROOM_PANEL_STYLE),
    ui.log("chat-log", message_lines(ctx, notices), CHAT_LOG_STYLE),
    ui.input("chat-message", tostring(setting(ctx, "input_placeholder", "/msg #station")), draft_value(ctx), "bottom"),
  })
  screen.scroll = "bottom"
  return screen
end

function view(ctx)
  return chat_screen(ctx, {})
end

function update(ctx, event)
  local notices = {}
  local action = event and event.action or ""
  local values = (event and event.values) or {}
  if event and event.kind == "submit" and event.target == "chat-message" then
    local text = trim(values["chat-message"] or "")
    if text ~= "" then
      if text:sub(1, 1) == "/" then
        handle_command(ctx, notices, text)
      else
        append_message(ctx, {
          from_public_key = public_key(ctx),
          from_fingerprint = fingerprint(ctx),
          from_display_name = display_name(ctx),
          text = text,
        })
      end
      ctx.store:set("user", "draft", "")
    end
  elseif action == "clear_room_log" then
    ctx.store:set("room", "messages", {})
    broadcast_room_changed(ctx)
    ctx.effects.notify("Channel backlog cleared.", "info", "room")
  end
  return chat_screen(ctx, notices)
end

function on_join(ctx)
  if setting(ctx, "join_leave_events", true) then
    append_message(ctx, {
      kind = "event",
      from = "system",
      text = display_name(ctx) .. " joined",
    })
    ctx.effects.notify(display_name(ctx) .. " joined chat.", "info", "room")
  end
  return chat_screen(ctx, {})
end

function on_leave(ctx)
  if setting(ctx, "join_leave_events", true) then
    append_message(ctx, {
      kind = "event",
      from = "system",
      text = display_name(ctx) .. " left",
    })
    ctx.effects.notify(display_name(ctx) .. " left chat.", "info", "room")
  end
  return chat_screen(ctx, {})
end
