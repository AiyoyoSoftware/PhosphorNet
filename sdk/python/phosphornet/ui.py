def screen(children, scroll="", capture_keys=False, style=None):
    result = {"component": "screen", "children": children}
    if scroll:
        result["scroll"] = scroll
    if capture_keys:
        result["capture_keys"] = True
    if style:
        result["style"] = style
    return result


def header(text):
    return {"component": "header", "text": text}


def text(value):
    return {"component": "text", "text": value}


def markdown(value):
    return {"component": "markdown", "text": value}


def status(value):
    return {"component": "status", "text": value}


def panel(title, children, style=None):
    result = {"component": "panel", "title": title, "children": children}
    if style:
        result["style"] = style
    return result


def menu(identifier, items):
    return {"component": "menu", "id": identifier, "items": items}


def list(identifier, items):
    return {"component": "list", "id": identifier, "items": items}


def dynamic_list(identifier, items):
    return {"component": "dynamic_list", "id": identifier, "items": items}


def item(label, action=""):
    result = {"label": label}
    if action:
        result["action"] = action
    return result


def button(identifier, label, action=""):
    result = {"component": "button", "id": identifier, "text": label}
    if action:
        result["action"] = action
    return result


def nav_button(identifier, label, view):
    return button(identifier, label, action=f"nav:push:{view}")


def back_button(identifier="back", label="Back"):
    return button(identifier, label, action="nav:back")


def checkbox(identifier, label, checked=False, action=""):
    result = {"component": "checkbox", "id": identifier, "text": label, "checked": bool(checked)}
    if action:
        result["action"] = action
    return result


def input(identifier, placeholder="", value="", dock=""):
    result = {"component": "input", "id": identifier}
    if placeholder:
        result["placeholder"] = placeholder
    if value:
        result["value"] = value
    if dock:
        result["dock"] = dock
    return result


def textarea(identifier, placeholder="", value="", dock=""):
    result = {"component": "textarea", "id": identifier}
    if placeholder:
        result["placeholder"] = placeholder
    if value:
        result["value"] = value
    if dock:
        result["dock"] = dock
    return result


def grid(identifier, rows):
    return {"component": "grid", "id": identifier, "rows": rows}


def log(identifier, children, style=None):
    result = {"component": "log", "id": identifier, "children": children}
    if style:
        result["style"] = style
    return result
