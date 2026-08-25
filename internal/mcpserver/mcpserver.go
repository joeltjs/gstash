package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/joeltjs/gstash/internal/git"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func textResult(s string) *mcp.CallToolResult {
	return mcp.NewToolResultText(s)
}

func errResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

func argsMap(req mcp.CallToolRequest) map[string]any {
	if m, ok := req.Params.Arguments.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func argInt(args map[string]any, key string) (int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("missing required param: %s", key)
	}
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	default:
		return 0, fmt.Errorf("param %s must be a number", key)
	}
}

func argBool(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func argString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func stashRef(n int) string {
	return fmt.Sprintf("stash@{%d}", n)
}

// Run serves the gstash MCP server over stdio. dir is the git repository the
// server operates on (normally the process working directory).
func Run(dir string) error {
	s := server.NewMCPServer(
		"gstash",
		"0.1.0",
		server.WithInstructions("Git stash tools. Read-only tools (list/show) are always safe. pop/drop/branch change repository state: only call them when the user explicitly asked, after showing the diff with 'show'."),
	)

	listTool := mcp.NewTool("stash_list",
		mcp.WithDescription("List stashes with branch labels and ages"),
		mcp.WithBoolean("all", mcp.Description("include stashes from every branch (default: current branch + unknown only)")),
	)
	s.AddTool(listTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entries, err := git.StashList(dir)
		if err != nil {
			return errResult(err), nil
		}
		cur, _ := git.CurrentBranch(dir)
		if !argBool(argsMap(req), "all") {
			entries = git.FilterByCurrent(entries, cur)
		}
		var sb strings.Builder
		for _, e := range entries {
			fmt.Fprintf(&sb, "%s [%s/%s] %s (%s)\n", e.Ref, e.Branch, e.Source, e.Message, e.Age)
		}
		if sb.Len() == 0 {
			sb.WriteString("no stashes\n")
		}
		sb.WriteString(fmt.Sprintf("current branch: %s", cur))
		return textResult(sb.String()), nil
	})

	showTool := mcp.NewTool("stash_show",
		mcp.WithDescription("Full diff of one stash"),
		mcp.WithNumber("index", mcp.Description("stash index, e.g. 0 for stash@{0}"), mcp.Required()),
	)
	s.AddTool(showTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idx, err := argInt(argsMap(req), "index")
		if err != nil {
			return errResult(err), nil
		}
		out, err := git.Show(dir, stashRef(idx))
		if err != nil {
			return errResult(err), nil
		}
		if len(out) > 60000 {
			out = out[:60000] + "\n...(truncated)"
		}
		return textResult(out), nil
	})

	saveTool := mcp.NewTool("stash_save",
		mcp.WithDescription("Create a new stash from current uncommitted changes"),
		mcp.WithString("message", mcp.Description("short message")),
		mcp.WithBoolean("include_untracked", mcp.Description("also stash untracked files")),
	)
	s.AddTool(saveTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := git.Save(dir, argString(argsMap(req), "message"), argBool(argsMap(req), "include_untracked"))
		if err != nil {
			return errResult(err), nil
		}
		return textResult(out), nil
	})

	applyTool := mcp.NewTool("stash_apply",
		mcp.WithDescription("Apply a stash without deleting it"),
		mcp.WithNumber("index", mcp.Required()),
	)
	s.AddTool(applyTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idx, err := argInt(argsMap(req), "index")
		if err != nil {
			return errResult(err), nil
		}
		out, err := git.Apply(dir, stashRef(idx))
		if err != nil {
			return errResult(err), nil
		}
		return textResult(out), nil
	})

	popTool := mcp.NewTool("stash_pop",
		mcp.WithDescription("Apply a stash then delete it (dashboard Accept button). Requires explicit user request"),
		mcp.WithNumber("index", mcp.Required()),
	)
	s.AddTool(popTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idx, err := argInt(argsMap(req), "index")
		if err != nil {
			return errResult(err), nil
		}
		out, err := git.Pop(dir, stashRef(idx))
		if err != nil {
			return errResult(err), nil
		}
		return textResult(out), nil
	})

	dropTool := mcp.NewTool("stash_drop",
		mcp.WithDescription("Permanently delete a stash without applying it (dashboard Reject button). Destructive: requires confirm=true and an explicit user request"),
		mcp.WithNumber("index", mcp.Required()),
		mcp.WithBoolean("confirm", mcp.Description("must be true"), mcp.Required()),
	)
	s.AddTool(dropTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if !argBool(argsMap(req), "confirm") {
			return errResult(fmt.Errorf("drop refused: pass confirm=true and make sure the user explicitly asked")), nil
		}
		idx, err := argInt(argsMap(req), "index")
		if err != nil {
			return errResult(err), nil
		}
		out, err := git.Drop(dir, stashRef(idx))
		if err != nil {
			return errResult(err), nil
		}
		return textResult(out), nil
	})

	branchTool := mcp.NewTool("stash_branch",
		mcp.WithDescription("Create a new branch from a stash and check it out"),
		mcp.WithNumber("index", mcp.Required()),
		mcp.WithString("name", mcp.Description("new branch name"), mcp.Required()),
	)
	s.AddTool(branchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argsMap(req)
		idx, err := argInt(args, "index")
		if err != nil {
			return errResult(err), nil
		}
		name := argString(args, "name")
		if name == "" {
			return errResult(fmt.Errorf("name required")), nil
		}
		out, err := git.BranchFromStash(dir, stashRef(idx), name)
		if err != nil {
			return errResult(err), nil
		}
		return textResult(out), nil
	})

	return server.ServeStdio(s)
}
