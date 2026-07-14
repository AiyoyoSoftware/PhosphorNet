local ui = phosphornet.ui

-- Both tables are intentionally fixed in door code. Client input can select one
-- of these semantic actions, but it can never provide a rule id or command.
local choices_by_action = {
  run_uptime = {
    rule_id = "demo-uptime",
    label = "Host uptime",
    selection = "uptime",
  },
  run_disk_usage = {
    rule_id = "demo-disk-usage",
    label = "Root disk usage",
    selection = "disk_usage",
  },
  run_kernel_version = {
    rule_id = "demo-kernel-version",
    label = "Kernel version",
    selection = "kernel_version",
  },
}

local choices_by_rule = {}
for _, choice in pairs(choices_by_action) do
  choices_by_rule[choice.rule_id] = choice
end

local function request_id(ctx, choice)
  return "action-demo:" .. choice.rule_id .. ":" .. tostring(ctx.session and ctx.session.id or "session")
end

local function clip_output(value)
  local text = tostring(value or "")
  if #text > 1600 then
    return string.sub(text, 1, 1600) .. "\n[output clipped by Action Workshop]"
  end
  if text == "" then
    return "(no output)"
  end
  return text
end

local function result_panel(ctx)
  local rule_id = tostring(ctx.store:get("user", "last_rule_id", ""))
  if rule_id == "" then
    return ui.panel("Last Result", {
      ui.text("No action has run in this session yet."),
    })
  end

  local children = {
    ui.text("Choice: " .. tostring(ctx.store:get("user", "last_label", rule_id))),
    ui.text("Rule: " .. rule_id),
    ui.text("Exit code: " .. tostring(ctx.store:get("user", "last_exit_code", -1))),
    ui.text("Stdout:\n" .. clip_output(ctx.store:get("user", "last_stdout", ""))),
  }
  local stderr = tostring(ctx.store:get("user", "last_stderr", ""))
  if stderr ~= "" then
    table.insert(children, ui.text("Stderr:\n" .. clip_output(stderr)))
  end
  local action_error = tostring(ctx.store:get("user", "last_error", ""))
  if action_error ~= "" then
    table.insert(children, ui.text("Error: " .. clip_output(action_error)))
  end
  return ui.panel(ctx.store:get("user", "last_ok", false) and "Last Result · Success" or "Last Result · Failed", children)
end

function view(ctx)
  return ui.screen({
    ui.header("ACTION WORKSHOP"),
    ui.panel("Typed Host Actions", {
      ui.text("Each button maps to one rule ID fixed in this door."),
      ui.text("The manifest and actiond TOML must both authorize that exact rule."),
      ui.text("Commands and argv come only from operator-owned TOML; this door sends JSON input on stdin."),
    }),
    ui.panel("Commands", {
      ui.button("run-uptime", "Show Host Uptime", "run_uptime"),
      ui.button("run-disk-usage", "Show Root Disk Usage", "run_disk_usage"),
      ui.button("run-kernel-version", "Show Kernel Version", "run_kernel_version"),
    }),
    result_panel(ctx),
    ui.status("Configure the demo-* rules in actiond.toml, then enable this door from Station Admin."),
  })
end

function update(ctx, event)
  if event and event.kind == "action_result" and event.action_result then
    local result = event.action_result
    local choice = choices_by_rule[tostring(result.rule_id or "")]
    if choice and tostring(result.request_id or "") == request_id(ctx, choice) then
      ctx.store:set("user", "last_rule_id", choice.rule_id)
      ctx.store:set("user", "last_label", choice.label)
      ctx.store:set("user", "last_ok", result.ok == true)
      ctx.store:set("user", "last_exit_code", tonumber(result.exit_code) or -1)
      ctx.store:set("user", "last_stdout", tostring(result.stdout or ""))
      ctx.store:set("user", "last_stderr", tostring(result.stderr or ""))
      ctx.store:set("user", "last_error", tostring(result.error or ""))
    end
    return view(ctx)
  end

  local choice = choices_by_action[event and event.action or ""]
  if choice then
    ctx.effects.action(choice.rule_id, request_id(ctx, choice), {
      source = "action_demo",
      selection = choice.selection,
    })
  end
  return view(ctx)
end
