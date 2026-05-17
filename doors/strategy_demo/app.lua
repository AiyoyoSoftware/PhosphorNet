local ui = phosphornet.ui

local function turn(ctx)
  return tonumber(ctx.store:get("room", "turn", 1)) or 1
end

local function players(ctx)
  local value = ctx.store:get("room", "players", {})
  if type(value) ~= "table" then
    value = {}
  end
  local result = {}
  local current = tostring(ctx.user and ctx.user.fingerprint or "unknown")
  local found = false
  for _, player in ipairs(value) do
    if #result < 4 then
      table.insert(result, player)
    end
    if player == current then
      found = true
    end
  end
  if not found and #result < 4 then
    table.insert(result, current)
  end
  return result
end

local function grid(ctx)
  local value = ctx.store:get("room", "grid", {})
  if type(value) ~= "table" then
    return {}
  end
  return value
end

local function grid_is_empty(value)
  for _ in pairs(value) do
    return false
  end
  return true
end

local function grid_rows(ctx)
  local current_grid = grid(ctx)
  local current_players = players(ctx)
  local marks = { "A", "B", "C", "D" }
  if grid_is_empty(current_grid) then
    for index, _ in ipairs(current_players) do
      current_grid[tostring(index - 1) .. "," .. tostring(index - 1)] = marks[index]
    end
  end

  local rows = {}
  for y = 0, 4 do
    local cells = {}
    for x = 0, 4 do
      table.insert(cells, current_grid[tostring(x) .. "," .. tostring(y)] or ".")
    end
    table.insert(rows, cells)
  end
  return rows
end

local function player_lines(ctx)
  local lines = {}
  for index, player in ipairs(players(ctx)) do
    table.insert(lines, ui.text(tostring(index) .. ". " .. tostring(player)))
  end
  return lines
end

local function broadcast_room_changed(ctx)
  ctx.effects.broadcast({ kind = "action", target = "strategy", action = "room_changed" }, "room")
end

function view(ctx)
  return ui.screen({
    ui.header("IRON ORCHARD"),
    ui.panel("Strategy Demo", {
      ui.text("Shared tactics-room proof using room-scoped state."),
      ui.text("Room: " .. tostring(ctx.room and ctx.room.id or "unknown")),
      ui.text("Current turn: " .. tostring(turn(ctx))),
    }),
    ui.panel("Players", player_lines(ctx)),
    ui.panel("Grid", { ui.grid("orchard-grid", grid_rows(ctx)) }),
    ui.panel("Commands", {
      ui.button("end-turn", "End Turn", "end_turn"),
      ui.button("claim-center", "Claim Center", "claim_center"),
      ui.button("reset-room", "Reset Room", "reset_room"),
    }),
    ui.status("Turn and grid changes broadcast to everyone in the strategy room."),
  })
end

function update(ctx, event)
  local action = event and event.action or ""
  if action == "end_turn" then
    ctx.store:set("room", "turn", turn(ctx) + 1)
    broadcast_room_changed(ctx)
  elseif action == "claim_center" then
    local current_grid = grid(ctx)
    local current_players = players(ctx)
    local current_player = tostring(ctx.user and ctx.user.fingerprint or "unknown")
    local marks = { "A", "B", "C", "D" }
    local mark = "?"
    for index, player in ipairs(current_players) do
      if player == current_player then
        mark = marks[index] or "?"
        break
      end
    end
    current_grid["2,2"] = mark
    ctx.store:set("room", "grid", current_grid)
    ctx.store:set("room", "players", current_players)
    broadcast_room_changed(ctx)
    ctx.effects.notify("Center tile claimed.", "info", "room")
  elseif action == "reset_room" then
    ctx.effects.replace_state("room", { turn = 1, players = players(ctx), grid = {} })
    broadcast_room_changed(ctx)
    ctx.effects.notify("Strategy room reset.", "info", "room")
  end
  return view(ctx)
end

function on_join(ctx)
  ctx.store:set("room", "players", players(ctx))
  broadcast_room_changed(ctx)
  ctx.effects.notify(tostring(ctx.user and ctx.user.fingerprint or "someone") .. " joined Iron Orchard.", "info", "room")
  return view(ctx)
end

function on_leave(ctx)
  broadcast_room_changed(ctx)
  ctx.effects.notify(tostring(ctx.user and ctx.user.fingerprint or "someone") .. " left Iron Orchard.", "info", "room")
  return view(ctx)
end
