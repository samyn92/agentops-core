/*
Agent Runtime — Fantasy (Go)

Built-in tools: bash, read, edit, write, grep, ls, glob, fetch.
Each implements the fantasy.AgentTool interface.
*/
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
)

// buildBuiltinTools returns the requested built-in tools.
func buildBuiltinTools(names []string) []fantasy.AgentTool {
	registry := map[string]fantasy.AgentTool{
		"bash":  newBashTool(),
		"read":  newReadTool(),
		"edit":  newEditTool(),
		"write": newWriteTool(),
		"grep":  newGrepTool(),
		"ls":    newLsTool(),
		"glob":  newGlobTool(),
		"fetch": newFetchTool(),
	}

	var tools []fantasy.AgentTool
	for _, name := range names {
		if t, ok := registry[name]; ok {
			tools = append(tools, t)
		}
	}
	return tools
}

// ── bash ──

type bashInput struct {
	Command string `json:"command" description:"The bash command to execute"`
	Timeout int    `json:"timeout,omitempty" description:"Timeout in seconds (default: 120)"`
}

func newBashTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("bash",
		"Execute a bash command. Returns stdout and stderr. Use for running programs, installing packages, file operations, and system tasks.",
		func(ctx context.Context, input bashInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.Command == "" {
				return fantasy.NewTextErrorResponse("command is required"), nil
			}
			cmd := exec.CommandContext(ctx, "bash", "-c", input.Command)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("%s\n%s", string(out), err.Error())), nil
			}
			return fantasy.NewTextResponse(string(out)), nil
		})
}

// ── read ──

type readInput struct {
	Path   string `json:"path" description:"Path to the file to read"`
	Offset int    `json:"offset,omitempty" description:"Line number to start reading from (1-indexed)"`
	Limit  int    `json:"limit,omitempty" description:"Maximum number of lines to read"`
}

func newReadTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("read",
		"Read the contents of a file. Supports text files. Use offset/limit for large files.",
		func(_ context.Context, input readInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.Path == "" {
				return fantasy.NewTextErrorResponse("path is required"), nil
			}
			data, err := os.ReadFile(input.Path)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			content := string(data)
			if input.Offset > 0 || input.Limit > 0 {
				lines := strings.Split(content, "\n")
				start := 0
				if input.Offset > 0 {
					start = input.Offset - 1
				}
				if start >= len(lines) {
					return fantasy.NewTextResponse(""), nil
				}
				end := len(lines)
				if input.Limit > 0 && start+input.Limit < end {
					end = start + input.Limit
				}
				content = strings.Join(lines[start:end], "\n")
			}
			return fantasy.NewTextResponse(content), nil
		})
}

// ── edit ──

type editEntry struct {
	OldText string `json:"oldText" description:"Exact text to find and replace"`
	NewText string `json:"newText" description:"Replacement text"`
}

type editInput struct {
	Path  string      `json:"path" description:"Path to the file to edit"`
	Edits []editEntry `json:"edits" description:"List of exact text replacements"`
}

func newEditTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("edit",
		"Edit a file using exact text replacement. Each edit's oldText must match a unique region of the file.",
		func(_ context.Context, input editInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.Path == "" {
				return fantasy.NewTextErrorResponse("path is required"), nil
			}
			data, err := os.ReadFile(input.Path)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			content := string(data)
			for _, e := range input.Edits {
				if !strings.Contains(content, e.OldText) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("oldText not found in file: %q", truncate(e.OldText, 80))), nil
				}
				if strings.Count(content, e.OldText) > 1 {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("oldText matches multiple locations: %q", truncate(e.OldText, 80))), nil
				}
				content = strings.Replace(content, e.OldText, e.NewText, 1)
			}
			if err := os.WriteFile(input.Path, []byte(content), 0644); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Applied %d edit(s) to %s", len(input.Edits), input.Path)), nil
		})
}

// ── write ──

type writeInput struct {
	Path    string `json:"path" description:"Path to the file to write"`
	Content string `json:"content" description:"Content to write to the file"`
}

func newWriteTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("write",
		"Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Automatically creates parent directories.",
		func(_ context.Context, input writeInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.Path == "" {
				return fantasy.NewTextErrorResponse("path is required"), nil
			}
			if err := os.MkdirAll(filepath.Dir(input.Path), 0755); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if err := os.WriteFile(input.Path, []byte(input.Content), 0644); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Wrote %d bytes to %s", len(input.Content), input.Path)), nil
		})
}

// ── grep ──

type grepInput struct {
	Pattern string `json:"pattern" description:"Regex pattern to search for"`
	Path    string `json:"path" description:"File or directory to search"`
}

func newGrepTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("grep",
		"Search for a regex pattern in files using ripgrep (rg). Returns matching lines with file paths and line numbers.",
		func(ctx context.Context, input grepInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.Pattern == "" {
				return fantasy.NewTextErrorResponse("pattern is required"), nil
			}
			path := input.Path
			if path == "" {
				path = "."
			}
			cmd := exec.CommandContext(ctx, "rg", "--line-number", "--no-heading", input.Pattern, path)
			out, _ := cmd.CombinedOutput()
			if len(out) == 0 {
				return fantasy.NewTextResponse("No matches found."), nil
			}
			return fantasy.NewTextResponse(string(out)), nil
		})
}

// ── ls ──

type lsInput struct {
	Path string `json:"path" description:"Directory path to list (defaults to current directory)"`
}

func newLsTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("ls",
		"List directory contents with file types and sizes.",
		func(_ context.Context, input lsInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			path := input.Path
			if path == "" {
				path = "."
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			var sb strings.Builder
			for _, e := range entries {
				info, _ := e.Info()
				if info != nil {
					fmt.Fprintf(&sb, "%s %8d %s\n", info.Mode(), info.Size(), e.Name())
				} else {
					fmt.Fprintf(&sb, "%s\n", e.Name())
				}
			}
			return fantasy.NewTextResponse(sb.String()), nil
		})
}

// ── glob ──

type globInput struct {
	Pattern string `json:"pattern" description:"Glob pattern to match files (e.g. **/*.go)"`
	Path    string `json:"path,omitempty" description:"Root directory to search from (defaults to current directory)"`
}

func newGlobTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("glob",
		"Find files matching a glob pattern. Returns matching file paths.",
		func(_ context.Context, input globInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.Pattern == "" {
				return fantasy.NewTextErrorResponse("pattern is required"), nil
			}
			root := input.Path
			if root == "" {
				root = "."
			}
			var matches []string
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				matched, _ := filepath.Match(input.Pattern, filepath.Base(path))
				if matched {
					matches = append(matches, path)
				}
				return nil
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if len(matches) == 0 {
				return fantasy.NewTextResponse("No files matched."), nil
			}
			return fantasy.NewTextResponse(strings.Join(matches, "\n")), nil
		})
}

// ── fetch ──

type fetchInput struct {
	URL string `json:"url" description:"URL to fetch"`
}

func newFetchTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("fetch",
		"Fetch the contents of a URL. Returns the response body as text.",
		func(ctx context.Context, input fetchInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if input.URL == "" {
				return fantasy.NewTextErrorResponse("url is required"), nil
			}
			cmd := exec.CommandContext(ctx, "curl", "-sSL", "--max-time", "30", input.URL)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("%s\n%s", string(out), err.Error())), nil
			}
			return fantasy.NewTextResponse(string(out)), nil
		})
}

// ── helpers ──

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
