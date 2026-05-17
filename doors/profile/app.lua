local ui = phosphornet.ui

local function trim(value)
  return tostring(value or ""):match("^%s*(.-)%s*$")
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

local function identity_panel(title, children)
  return ui.panel(title, children, gradient_background("vertical", {
    { at = 0.0, color = "#101827" },
    { at = 0.55, color = "#1f1b4d" },
    { at = 1.0, color = "#17313f" },
  }))
end

local function help_panel(title, children)
  return ui.panel(title, children, gradient_background("diagonal", {
    { at = 0.0, color = "#111827" },
    { at = 0.55, color = "#253047" },
    { at = 1.0, color = "#1f3a2f" },
  }))
end

local function guest_name(fingerprint)
  local value = tostring(fingerprint or "unknown")
  local first = value:match("^[^-]+") or value
  if first == "" then
    first = "unknown"
  end
  return "guest-" .. first
end

local function current_display_name(ctx)
  local name = trim(ctx.user and ctx.user.display_name or "")
  if name ~= "" then
    return name
  end
  return guest_name(ctx.user and ctx.user.fingerprint)
end

local function profile_screen(ctx)
  local children = {
    ui.header("PROFILE"),
    identity_panel("Station Identity", {
      ui.text("Passport: " .. (ctx.user.fingerprint or "unknown")),
      ui.text("Current display name: " .. current_display_name(ctx)),
      ui.input("profile-display-name", setting(ctx, "display_name_placeholder", "display name"), trim(ctx.user.display_name or "")),
      ui.input("profile-status-line", setting(ctx, "status_line_placeholder", "status line"), trim(ctx.user.status_line or "")),
      ui.textarea("profile-bio", setting(ctx, "bio_placeholder", "optional bio"), trim(ctx.user.bio or "")),
      ui.button("save-profile", "Save Profile", "save_profile"),
      ui.button("reset-profile", "Reset Profile", "reset_profile"),
    }),
  }
  if setting(ctx, "show_passport_help", true) then
    table.insert(children, help_panel("About Passports", {
      ui.text("Your passport fingerprint is the actual identity anchor."),
      ui.text("The display name is the friendly station layer shown in lobby, chat, forum, and admin views."),
    }))
  end
  table.insert(children, ui.status("Display name is local to this station. Passport fingerprint remains authoritative."))
  return ui.screen(children)
end

function view(ctx)
  return profile_screen(ctx)
end

function update(ctx, event)
  local action = event and event.action or ""
  local values = (event and event.values) or {}
  if action == "save_profile" then
    ctx.effects.update_profile(
      values["profile-display-name"] or ctx.user.display_name or "",
      values["profile-bio"] or ctx.user.bio or "",
      values["profile-status-line"] or ctx.user.status_line or ""
    )
    ctx.effects.notify("Profile saved.", "info", "self")
  elseif action == "reset_profile" then
    ctx.effects.reset_profile()
    ctx.effects.notify("Profile reset.", "warning", "self")
  end
  return profile_screen(ctx)
end
