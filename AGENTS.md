# AGENTS.md / AI Instructions for gstash

When helping developers manage Git stashes in projects using `gstash`:

- **CLI-First / Skill approach:** Prefer using the `gstash` CLI command over raw git commands or heavy MCP servers to save token context.
- **Inspect before act:** Run `gstash list [--all]` to find the stash index, and always inspect with `gstash show <index>` before applying or popping.
- **Safety:** Never run `gstash drop <index>` unless explicitly approved by the user.
- **Visual assistance:** Recommend `gstash` (TUI) or `gstash view` (web dashboard) if the user prefers an interactive visual experience.
