local ui = phosphornet.ui

local SEED_TITLE = "Welcome to PhosphorNet"
local SEED_BODY = table.concat({
  "## Rules / vibe / how to use this station",
  "",
  "- Be kind and keep the place readable.",
  "- Post in markdown when you want a longer thought to land well.",
  "- Chat is for fast back-and-forth. The forum is for slower, lasting conversations.",
  "",
  "## What are doors?",
  "",
  "Doors are hosted experiences inside a station. They can be chats, boards, tools, or little places with a clear purpose.",
  "",
  "If you are new here, read the thread list and then jump into chat.",
}, "\n")

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

local FORUM_PANEL_STYLES = {
  list = gradient_background("vertical", {
    { at = 0.0, color = "#121827" },
    { at = 0.55, color = "#1e2451" },
    { at = 1.0, color = "#263238" },
  }),
  post = gradient_background("vertical", {
    { at = 0.0, color = "#171322" },
    { at = 0.5, color = "#2a2148" },
    { at = 1.0, color = "#362a18" },
  }),
  action = gradient_background("diagonal", {
    { at = 0.0, color = "#10243a" },
    { at = 0.55, color = "#172f40" },
    { at = 1.0, color = "#243522" },
  }),
  compose = gradient_background("vertical", {
    { at = 0.0, color = "#161827" },
    { at = 0.55, color = "#2b244a" },
    { at = 1.0, color = "#3b241d" },
  }),
  notice = gradient_background("diagonal", {
    { at = 0.0, color = "#1f1722" },
    { at = 0.55, color = "#2d2438" },
    { at = 1.0, color = "#352a18" },
  }),
}

local function forum_panel(title, children, variant)
  return ui.panel(title, children, FORUM_PANEL_STYLES[variant or "list"] or FORUM_PANEL_STYLES.list)
end

local function trim(value)
  return tostring(value or ""):match("^%s*(.-)%s*$")
end

local function is_admin(ctx)
  local role = tostring(ctx.user and ctx.user.role or "")
  return role == "admin" or role == "sysop"
end

local function maintenance_mode(ctx)
  return ctx.node and ctx.node.maintenance_mode and true or false
end

local function posting_locked(ctx)
  return maintenance_mode(ctx) and not is_admin(ctx) and not setting(ctx, "allow_posts_during_maintenance", false)
end

local function display_name(ctx)
  local name = trim(ctx.user and ctx.user.display_name or "")
  if name ~= "" then
    return name
  end
  local fingerprint = tostring(ctx.user and ctx.user.fingerprint or "unknown")
  local first = fingerprint:match("^[^-]+") or fingerprint
  return "guest-" .. first
end

local function display_name_for_profile(profile)
  local name = trim(profile and profile.display_name or "")
  if name ~= "" then
    return name
  end
  local fingerprint = tostring(profile and profile.fingerprint or "unknown")
  local first = fingerprint:match("^[^-]+") or fingerprint
  return "guest-" .. first
end

local function profile_by_public_key(ctx, public_key)
  if public_key == nil or public_key == "" then
    return nil
  end
  for _, user in ipairs(ctx.users or {}) do
    if user.public_key == public_key then
      return user
    end
  end
  return nil
end

local function post_author_name(ctx, post)
  if post.author_public_key and post.author_public_key ~= "" then
    local profile = profile_by_public_key(ctx, post.author_public_key)
    if profile then
      return display_name_for_profile(profile)
    end
  end
  local author = trim(post.author or "")
  if author ~= "" then
    return author
  end
  if post.author_fingerprint and post.author_fingerprint ~= "" then
    return "guest-" .. ((tostring(post.author_fingerprint):match("^[^-]+")) or tostring(post.author_fingerprint))
  end
  return "unknown"
end

local function copy_array(values)
  local result = {}
  for index, value in ipairs(values or {}) do
    result[index] = value
  end
  return result
end

local function room_threads(ctx)
  local value = ctx.store:get("room", "threads", {})
  if type(value) ~= "table" then
    return {}
  end
  return value
end

local function room_posts(ctx)
  local value = ctx.store:get("room", "posts", {})
  if type(value) ~= "table" then
    return {}
  end
  return value
end

local function set_room_threads(ctx, threads)
  ctx.store:set("room", "threads", threads)
end

local function set_room_posts(ctx, posts)
  ctx.store:set("room", "posts", posts)
end

local function next_room_number(ctx, key, fallback)
  return tonumber(ctx.store:get("room", key, fallback or 1)) or (fallback or 1)
end

local function set_room_number(ctx, key, value)
  ctx.store:set("room", key, tonumber(value) or 0)
end

local function allocate_id(ctx, key)
  local value = next_room_number(ctx, key, 1)
  set_room_number(ctx, key, value + 1)
  return value
end

local function allocate_seq(ctx)
  return allocate_id(ctx, "next_activity_seq")
end

local function ensure_seed_data(ctx)
  local threads = room_threads(ctx)
  if #threads > 0 then
    return
  end

  local seq = allocate_seq(ctx)
  local thread_id = allocate_id(ctx, "next_thread_id")
  local post_id = allocate_id(ctx, "next_post_id")

  set_room_threads(ctx, {
    {
      id = thread_id,
      title = setting(ctx, "welcome_thread_title", SEED_TITLE),
      pinned = true,
      starter_post_id = post_id,
      created_seq = seq,
      updated_seq = seq,
    },
  })
  set_room_posts(ctx, {
    {
      id = post_id,
      thread_id = thread_id,
      author = "station",
      author_public_key = "",
      author_fingerprint = "station",
      body = setting(ctx, "welcome_thread_body", SEED_BODY),
      created_seq = seq,
      hidden = false,
    },
  })
end

local function normalize_counters(ctx)
  if tonumber(ctx.store:get("room", "next_thread_id", 0)) <= 0 then
    local max_id = 0
    for _, thread in ipairs(room_threads(ctx)) do
      max_id = math.max(max_id, tonumber(thread.id or 0) or 0)
    end
    set_room_number(ctx, "next_thread_id", max_id + 1)
  end
  if tonumber(ctx.store:get("room", "next_post_id", 0)) <= 0 then
    local max_id = 0
    for _, post in ipairs(room_posts(ctx)) do
      max_id = math.max(max_id, tonumber(post.id or 0) or 0)
    end
    set_room_number(ctx, "next_post_id", max_id + 1)
  end
  if tonumber(ctx.store:get("room", "next_activity_seq", 0)) <= 0 then
    local max_seq = 0
    for _, thread in ipairs(room_threads(ctx)) do
      max_seq = math.max(max_seq, tonumber(thread.updated_seq or thread.created_seq or 0) or 0)
    end
    for _, post in ipairs(room_posts(ctx)) do
      max_seq = math.max(max_seq, tonumber(post.created_seq or 0) or 0)
    end
    set_room_number(ctx, "next_activity_seq", max_seq + 1)
  end
end

local function thread_posts(ctx, thread_id)
  local posts = {}
  for _, post in ipairs(room_posts(ctx)) do
    if tonumber(post.thread_id or 0) == tonumber(thread_id or 0) then
      table.insert(posts, post)
    end
  end
  table.sort(posts, function(left, right)
    local left_seq = tonumber(left.created_seq or left.id or 0) or 0
    local right_seq = tonumber(right.created_seq or right.id or 0) or 0
    if left_seq == right_seq then
      return tonumber(left.id or 0) < tonumber(right.id or 0)
    end
    return left_seq < right_seq
  end)
  return posts
end

local function find_thread(ctx, thread_id)
  for _, thread in ipairs(room_threads(ctx)) do
    if tonumber(thread.id or 0) == tonumber(thread_id or 0) then
      return thread
    end
  end
  return nil
end

local function find_post(ctx, post_id)
  for _, post in ipairs(room_posts(ctx)) do
    if tonumber(post.id or 0) == tonumber(post_id or 0) then
      return post
    end
  end
  return nil
end

local function thread_starter_post(ctx, thread)
  if not thread then
    return nil
  end
  local starter_id = tonumber(thread.starter_post_id or 0) or 0
  if starter_id > 0 then
    local starter = find_post(ctx, starter_id)
    if starter then
      return starter
    end
  end
  local posts = thread_posts(ctx, tonumber(thread.id or 0) or 0)
  if #posts > 0 then
    return posts[1]
  end
  return nil
end

local function thread_reply_posts(ctx, thread)
  local replies = {}
  local starter_id = tonumber(thread and thread.starter_post_id or 0) or 0
  for _, post in ipairs(thread_posts(ctx, tonumber(thread and thread.id or 0) or 0)) do
    if tonumber(post.id or 0) ~= starter_id then
      table.insert(replies, post)
    end
  end
  return replies
end

local function parse_page(page)
  local parts = {}
  for part in tostring(page or "home"):gmatch("[^:]+") do
    table.insert(parts, part)
  end
  return parts
end

local function page_kind(ctx)
  return parse_page(ctx.nav:current("home"))
end

local function current_thread_id(ctx)
  local parts = page_kind(ctx)
  if parts[1] == "thread" or parts[1] == "reply" or parts[1] == "confirm" then
    return tonumber(parts[#parts] or 0) or 0
  end
  return 0
end

local function current_confirm_mode(ctx)
  local parts = page_kind(ctx)
  if parts[1] ~= "confirm" then
    return "", 0
  end
  return tostring(parts[2] or ""), tonumber(parts[3] or 0) or 0
end

local function thread_label(thread, author)
  local label = ""
  if thread.pinned then
    label = label .. "[pinned] "
  end
  label = label .. tostring(thread.title or "Untitled")
  return label
end

local function sorted_threads(ctx)
  local threads = copy_array(room_threads(ctx))
  local posts = room_posts(ctx)
  local reply_counts = {}
  for _, post in ipairs(posts) do
    local thread_id = tonumber(post.thread_id or 0) or 0
    reply_counts[thread_id] = (reply_counts[thread_id] or 0) + 1
  end
  for _, thread in ipairs(threads) do
    local thread_id = tonumber(thread.id or 0) or 0
    thread.reply_count = reply_counts[thread_id] or 0
  end
  table.sort(threads, function(left, right)
    if left.pinned ~= right.pinned then
      return left.pinned and not right.pinned
    end
    local left_seq = tonumber(left.updated_seq or left.created_seq or 0) or 0
    local right_seq = tonumber(right.updated_seq or right.created_seq or 0) or 0
    if left_seq == right_seq then
      return tonumber(left.id or 0) > tonumber(right.id or 0)
    end
    return left_seq > right_seq
  end)
  return threads
end

local function thread_list_items(ctx)
  local items = {}
  for _, thread in ipairs(sorted_threads(ctx)) do
    local label = thread_label(thread)
    table.insert(items, ui.item(label, "nav:push:thread:" .. tostring(thread.id)))
  end
  return items
end

local function draft_value(ctx, key)
  return tostring(ctx.store:get("user", key, "") or "")
end

local function remember_draft(ctx, key, value)
  ctx.store:set("user", key, tostring(value or ""))
end

local function clear_thread_drafts(ctx)
  ctx.store:set("user", "draft_thread_title", "")
  ctx.store:set("user", "draft_thread_body", "")
end

local function clear_reply_drafts(ctx)
  ctx.store:set("user", "draft_reply_body", "")
end

local function draft_notice(ctx, message)
  ctx.effects.notify(message, "warn", "self")
end

local function create_thread(ctx, values)
  if posting_locked(ctx) then
    draft_notice(ctx, "Forum is read-only during maintenance.")
    return
  end

  local title = trim(values["forum-thread-title"] or "")
  local body = trim(values["forum-thread-body"] or "")
  if title == "" or body == "" then
    remember_draft(ctx, "draft_thread_title", title)
    remember_draft(ctx, "draft_thread_body", body)
    draft_notice(ctx, "Thread title and body are required.")
    return
  end

  local threads = room_threads(ctx)
  local posts = room_posts(ctx)
  local thread_id = allocate_id(ctx, "next_thread_id")
  local post_id = allocate_id(ctx, "next_post_id")
  local seq = allocate_seq(ctx)
  table.insert(threads, {
    id = thread_id,
    title = title,
    pinned = false,
    starter_post_id = post_id,
    created_seq = seq,
    updated_seq = seq,
  })
  table.insert(posts, {
    id = post_id,
    thread_id = thread_id,
    author = display_name(ctx),
    author_public_key = ctx.user.public_key or "",
    author_fingerprint = ctx.user.fingerprint or "",
    body = body,
    created_seq = seq,
    hidden = false,
  })

  set_room_threads(ctx, threads)
  set_room_posts(ctx, posts)
  clear_thread_drafts(ctx)
  ctx.nav:reset("thread:" .. tostring(thread_id))
  ctx.effects.notify("Thread published.", "info", "self")
end

local function reply_to_thread(ctx, thread_id, values)
  if posting_locked(ctx) then
    draft_notice(ctx, "Forum is read-only during maintenance.")
    return
  end

  local body = trim(values["forum-reply-body"] or "")
  if body == "" then
    remember_draft(ctx, "draft_reply_body", body)
    draft_notice(ctx, "Reply body is required.")
    return
  end

  local thread = find_thread(ctx, thread_id)
  if not thread then
    draft_notice(ctx, "That thread no longer exists.")
    ctx.nav:reset("threads")
    return
  end

  local posts = room_posts(ctx)
  local post_id = allocate_id(ctx, "next_post_id")
  local seq = allocate_seq(ctx)
  table.insert(posts, {
    id = post_id,
    thread_id = tonumber(thread_id or 0),
    author = display_name(ctx),
    author_public_key = ctx.user.public_key or "",
    author_fingerprint = ctx.user.fingerprint or "",
    body = body,
    created_seq = seq,
    hidden = false,
  })
  thread.updated_seq = seq

  set_room_posts(ctx, posts)
  set_room_threads(ctx, room_threads(ctx))
  clear_reply_drafts(ctx)
  ctx.nav:reset("thread:" .. tostring(thread_id))
  ctx.effects.notify("Reply posted.", "info", "self")
end

local function hide_post(ctx, post_id)
  if not is_admin(ctx) then
    draft_notice(ctx, "Only admins can hide posts.")
    return
  end

  local posts = room_posts(ctx)
  local updated = false
  for _, post in ipairs(posts) do
    if tonumber(post.id or 0) == tonumber(post_id or 0) then
      post.hidden = true
      updated = true
      break
    end
  end
  if not updated then
    draft_notice(ctx, "That post no longer exists.")
    return
  end
  set_room_posts(ctx, posts)
  ctx.nav:back("threads")
  ctx.effects.notify("Post hidden.", "info", "self")
end

local function delete_post(ctx, post_id)
  if not is_admin(ctx) then
    draft_notice(ctx, "Only admins can delete posts.")
    return
  end

  local posts = room_posts(ctx)
  local next_posts = {}
  local removed = false
  for _, post in ipairs(posts) do
    if tonumber(post.id or 0) ~= tonumber(post_id or 0) then
      table.insert(next_posts, post)
    else
      removed = true
    end
  end
  if not removed then
    draft_notice(ctx, "That post no longer exists.")
    return
  end
  set_room_posts(ctx, next_posts)
  ctx.nav:back("threads")
  ctx.effects.notify("Post deleted.", "info", "self")
end

local function render_home(ctx)
  local threads = sorted_threads(ctx)

  local thread_items = {}
  for _, thread in ipairs(threads) do
    table.insert(thread_items, ui.item(thread_label(thread), "nav:push:thread:" .. tostring(thread.id)))
  end

  local children = {
    ui.header("FORUM"),
    forum_panel("Thread List", {
      ui.menu("forum-thread-list-home", thread_items),
    }, "list"),
    ui.button("forum-new-thread", "New Thread", "nav:push:new"),
  }
  table.insert(children, ui.status(posting_locked(ctx) and "Forum is read-only during maintenance." or "Ready."))
  return ui.screen(children)
end

local function render_threads(ctx)
  local items = thread_list_items(ctx)
  if #items == 0 then
    items = { ui.item(setting(ctx, "empty_thread_action", "No threads yet."), "nav:push:new") }
  end
  return ui.screen({
    ui.header("FORUM / THREADS"),
    forum_panel("Latest Activity", {
      ui.menu("forum-thread-list", items),
    }, "list"),
    forum_panel("Actions", {
      ui.nav_button("forum-threads-home", "Home", "home"),
      ui.nav_button("forum-threads-new", "New Thread", "new"),
    }, "action"),
    ui.status("Pinned threads stay first; the newest activity rises to the top."),
  })
end

local function render_post(ctx, post)
  local author = post_author_name(ctx, post)
  local fingerprint = tostring(post.author_fingerprint or "unknown")
  local prefix = author .. " · " .. fingerprint .. " · post #" .. tostring(post.id or 0)
  if post.hidden then
    prefix = prefix .. " [hidden]"
  end
  local children = { ui.text(prefix) }
  if post.hidden then
    table.insert(children, ui.text("[hidden by sysop]"))
  else
    table.insert(children, ui.markdown(tostring(post.body or "")))
  end
  if is_admin(ctx) then
    table.insert(children, ui.text("Moderation"))
    if not post.hidden then
      table.insert(children, ui.button("hide-post-" .. tostring(post.id), "Hide", "nav:push:confirm:hide:" .. tostring(post.id)))
    end
    table.insert(children, ui.button("delete-post-" .. tostring(post.id), "Delete", "nav:push:confirm:delete:" .. tostring(post.id)))
  end
  return forum_panel("Post #" .. tostring(post.id or 0), children, "post")
end

local function render_thread(ctx, thread_id)
  local thread = find_thread(ctx, thread_id)
  if not thread then
    return ui.screen({
      ui.header("FORUM / THREAD"),
      forum_panel("Missing", {
        ui.text("That thread could not be found."),
      }, "notice"),
      ui.nav_button("forum-thread-back", "Back", "threads"),
    })
  end

  local starter_post = thread_starter_post(ctx, thread)
  local replies = thread_reply_posts(ctx, thread)
  local children = {
    ui.header("FORUM / " .. tostring(thread.title or "Untitled")),
    ui.text("Posts: " .. tostring(#thread_posts(ctx, thread_id))),
  }
  if setting(ctx, "show_activity_sequence", true) then
    table.insert(children, ui.text("Latest activity sequence: " .. tostring(thread.updated_seq or thread.created_seq or 0)))
  end

  if starter_post then
    table.insert(children, render_post(ctx, starter_post))
  end

  if #replies > 0 then
    table.insert(children, forum_panel("Replies", {
      ui.text(tostring(#replies) .. " " .. (#replies == 1 and "reply" or "replies")),
    }, "notice"))
  end

  for _, post in ipairs(replies) do
    table.insert(children, render_post(ctx, post))
  end

  table.insert(children, forum_panel("Actions", {
    ui.button("forum-thread-reply", "Reply", "nav:push:reply:" .. tostring(thread_id)),
    ui.nav_button("forum-thread-back", "Back to list", "threads"),
  }, "action"))
  table.insert(children, ui.status(posting_locked(ctx) and "Read-only during maintenance." or "Replies render locally as markdown."))
  return ui.screen(children)
end

local function render_new_thread(ctx)
  return ui.screen({
    ui.header("FORUM / NEW THREAD"),
    forum_panel("Compose", {
      ui.input("forum-thread-title", "Thread title", draft_value(ctx, "draft_thread_title")),
      ui.textarea("forum-thread-body", "Write the first post in markdown", draft_value(ctx, "draft_thread_body")),
      ui.button("forum-create-thread", "Publish Thread", "create_thread"),
      ui.back_button("forum-new-back"),
    }, "compose"),
    ui.status(posting_locked(ctx) and "Forum is read-only during maintenance." or "The first post becomes the thread starter."),
  })
end

local function render_reply(ctx, thread_id)
  local thread = find_thread(ctx, thread_id)
  if not thread then
    return ui.screen({
      ui.header("FORUM / REPLY"),
      forum_panel("Missing", {
        ui.text("That thread could not be found."),
      }, "notice"),
      ui.nav_button("forum-reply-back", "Back", "threads"),
    })
  end

  return ui.screen({
    ui.header("FORUM / REPLY"),
    forum_panel("Replying To", {
      ui.text(tostring(thread.title or "Untitled")),
      ui.text("Replies are appended to the thread and rendered locally as markdown."),
      ui.text("Signed in as " .. display_name(ctx)),
    }, "notice"),
    ui.nav_button("forum-reply-context", "Back to thread", "thread:" .. tostring(thread_id)),
    forum_panel("Compose", {
      ui.textarea("forum-reply-body", "Write your reply in markdown", draft_value(ctx, "draft_reply_body")),
      ui.button("forum-post-reply", "Post Reply", "post_reply"),
      ui.back_button("forum-reply-back"),
    }, "compose"),
    ui.status(posting_locked(ctx) and "Forum is read-only during maintenance." or "Use markdown for longer replies and notes."),
  })
end

local function render_confirm(ctx)
  local mode, post_id = current_confirm_mode(ctx)
  local post = find_post(ctx, post_id)
  local action_label = mode == "hide" and "hide" or "delete"
  local title = post and ("Post #" .. tostring(post.id or post_id)) or "Missing post"
  local body = {
    ui.header("FORUM / CONFIRM"),
    forum_panel("Confirm " .. action_label, {
      ui.text("You are about to " .. action_label .. " a forum post."),
      ui.text(title),
      ui.text(post and ("Author: " .. post_author_name(ctx, post)) or "That post no longer exists."),
    }, "notice"),
    forum_panel("Actions", {
      ui.back_button("forum-confirm-back"),
    }, "action"),
  }

  if post and mode == "hide" then
    table.insert(body[3].children, ui.button("forum-confirm-hide", "Confirm Hide", "confirm_hide_post"))
  elseif post and mode == "delete" then
    table.insert(body[3].children, ui.button("forum-confirm-delete", "Confirm Delete", "confirm_delete_post"))
  end

  table.insert(body, ui.status("Moderation actions are limited to admins and sysops."))
  return ui.screen(body)
end

function view(ctx)
  ensure_seed_data(ctx)
  normalize_counters(ctx)

  local page = ctx.nav:current("home")
  if page == "threads" then
    return render_threads(ctx)
  end
  if page == "new" then
    return render_new_thread(ctx)
  end
  if page:match("^thread:") then
    return render_thread(ctx, tonumber(page:match("^thread:(%d+)$") or "0") or 0)
  end
  if page:match("^reply:") then
    return render_reply(ctx, tonumber(page:match("^reply:(%d+)$") or "0") or 0)
  end
  if page:match("^confirm:") then
    return render_confirm(ctx)
  end
  return render_home(ctx)
end

function update(ctx, event)
  ensure_seed_data(ctx)
  normalize_counters(ctx)

  if ctx.nav:handle(event, "home") then
    return view(ctx)
  end

  local action = event and event.action or ""
  local values = (event and event.values) or {}

  if action == "create_thread" then
    create_thread(ctx, values)
  elseif action == "post_reply" then
    local thread_id = current_thread_id(ctx)
    if thread_id > 0 then
      reply_to_thread(ctx, thread_id, values)
    else
      draft_notice(ctx, "Open a thread before replying.")
    end
  elseif action == "confirm_hide_post" then
    local mode, post_id = current_confirm_mode(ctx)
    if mode == "hide" then
      hide_post(ctx, post_id)
    end
  elseif action == "confirm_delete_post" then
    local mode, post_id = current_confirm_mode(ctx)
    if mode == "delete" then
      delete_post(ctx, post_id)
    end
  elseif string.sub(action, 1, 10) == "open_door:" then
    local door_id = string.sub(action, 11)
    if door_id ~= "" then
      ctx.effects.transition("open_door", door_id)
    end
  elseif action == "reset_drafts" then
    clear_thread_drafts(ctx)
    clear_reply_drafts(ctx)
  end

  return view(ctx)
end

function on_join(ctx)
  ensure_seed_data(ctx)
  normalize_counters(ctx)
  ctx.nav:reset("home")
  return render_home(ctx)
end
