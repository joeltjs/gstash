# gstash

A git stash manager with a visual terminal UI and a local web dashboard. Filter stashes by branch, preview diffs, then apply, pop, drop or branch from one screen.

## Install

Requires Go 1.22 or newer.

```bash
git clone https://github.com/joeltjs/gstash.git
cd gstash
go install .
```

The `gstash` binary lands in `$GOPATH/bin` (usually `~/go/bin`). Make sure that directory is on your `PATH`.

## Quick start

```bash
cd any-git-repo

# stash something first (or edit files, then use the TUI)
echo "wip" >> file.txt
gstash save "wip login page"

gstash                # open the terminal UI
```

Inside the TUI:

```text
↑/↓ select · tab filter · n save-new · a apply · p pop · d drop
b branch · v view (dashboard) · ? help · r refresh · q quit
```

Left panel: the stash list with branch labels and ages. Right panel: the diff of the selected stash (scroll with `pgup`/`pgdn`). `tab` toggles the filter between *current branch* and *all branches*. Press `?` for the full key reference.

The **SRC** column shows how trustworthy the branch label is: `✓` exact (recorded by git in the reflog), `~` inferred from the parent commit, `?` unknown. Hide it with the SRC checkbox in the dashboard header; in the TUI it is part of the branch column.

## Commands

| Command | What it does |
|---|---|
| `gstash` | Interactive terminal UI |
| `gstash view [--port N]` | Web dashboard: stash table, diff pane, action buttons. Port comes from `GSTASH_DASHBOARD_PORT` in `.env` (example default: 3889) |
| `gstash list [--all]` | List stashes, filtered to the current branch by default |
| `gstash show <index>` | Full diff of a stash |
| `gstash save [message] [-u]` | Create a stash (`-u` includes untracked files); records the branch name in the message |
| `gstash apply <index>` | Apply a stash without deleting it |
| `gstash pop <index>` | Apply and delete |
| `gstash drop <index> [-y]` | Delete a stash (asks for confirmation) |
| `gstash branch <index> [name]` | Create a new branch from a stash |

## Inspecting stashes

```bash
$ gstash list --all
Stashes on all branches:

  ID   BRANCH              AGE             MESSAGE
  #0   main                2 minutes ago   wip login page
  #1   main                5 hours ago     [branch:main] eksperimen validator

~ = inferred from commits, ? = unknown

$ gstash show 0        # full diff of stash@{0}
```

In the TUI the diff appears in the right panel as soon as you select a stash. In the dashboard click a row and the diff opens in the bottom panel next to Accept / Reject / Branch buttons.

## Dashboard

```bash
gstash view
```

Serves a dashboard on loopback and opens your browser. The port is read from `GSTASH_DASHBOARD_PORT` in `.env` (example default: 3889), or pass `--port`.

Keep it alive after closing the terminal:

```bash
nohup gstash view > /tmp/gstash-view.log 2>&1 &

# check logs / stop it
tail -f /tmp/gstash-view.log
pkill -f "gstash view"
```

There is no `npm run dev` and no Docker: the whole UI is embedded in the Go binary. Install once, run anywhere.

### Accept / Reject

| Button | Git command | Effect |
|---|---|---|
| **Accept · pop** | `git stash pop` | Changes land in the working tree; the stash is removed from the list |
| **Reject · drop** | `git stash drop` | The stash is deleted permanently without applying anything (a dialog asks first) |
| **Branch** | `git stash branch <name>` | A new branch is created from the stash and checked out |

## Branch labels

Git stores no branch information inside a stash, so gstash uses three layers:

1. **Exact** — git itself writes `On main:` / `WIP on main:` into the stash reflog. Trusted fully.
2. **Prefix** — stashes created by `gstash save` embed `[branch:<name>]` in the message.
3. **Inferred (~)** — derived from the stash parent commit via `git branch --contains`.
4. **Unknown (?)** — always shown so nothing silently disappears.

The default filter only shows stashes belonging to the current branch (plus unknown ones).

## AI Agent Integration (Skill & MCP)

### Agent Skill / System Prompt (Recommended)
To save context tokens and avoid MCP overhead, prefer the **CLI-First / Skill** approach. Modern coding agents (Kilo, Claude Code, Cursor, Windsurf) can directly read `AGENTS.md` or the skills provided in `.agents/skills/gstash/SKILL.md` (or `.kilo/skills/gstash/SKILL.md`).

Quick rules for agents / system prompts:
- Use `gstash list [--all]` to inspect stashes with branch labels and origins.
- Use `gstash show <index>` to preview diffs before applying or popping.
- Never run `gstash drop` without explicit user confirmation.

### MCP Server (Optional)
For clients that specifically require formal Model Context Protocol support:

```json
{
  "mcpServers": {
    "gstash": { "command": "gstash", "args": ["mcp"] }
  }
}
```

The config file location depends on the client: Claude Desktop uses `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS and `%APPDATA%\Claude\claude_desktop_config.json` on Windows; Kilo and most CLI agents accept the same JSON in their own MCP settings. The spawned process must run inside the target repository — launch your agent from the repo folder, or set `cwd` if the client supports it.

Tools: `stash_list`, `stash_show`, `stash_save`, `stash_apply`, `stash_pop`, `stash_branch`, and `stash_drop` (requires `confirm: true`). The server instructions forbid destructive calls without an explicit user request.

### Debugging

Test the server before blaming the agent:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' | gstash mcp
```

One JSON line containing `"result"` means the server is alive. For point-and-click testing:

```bash
npx @modelcontextprotocol/inspector gstash mcp
```

| Symptom | Cause and fix |
|---|---|
| `Error: not a git repository` | The process runs outside a repo. Launch the agent from inside one |
| Tools appear but every call fails | The process cwd is not the target repository |
| Server exits immediately | Binary not on PATH. Run `go install .` again |

## Security notes

The dashboard binds to `127.0.0.1` only, validates the `Host` header against a loopback allowlist (DNS-rebinding protection), rejects state-changing requests with a foreign `Origin`, requires a custom `X-Requested-With` header that cross-site forms cannot send, sends a restrictive CSP, and validates every `ref` against a strict pattern before touching git. There is no authentication by design: anyone who can reach the port already has local access. Do not tunnel it to a network.

## License

MIT.
