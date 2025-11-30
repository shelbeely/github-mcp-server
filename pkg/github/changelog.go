package github

import (
	"context"
	"encoding/json"
	"fmt"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ChangelogEntry represents a normalized commit entry for changelog generation.
type ChangelogEntry struct {
	SHA         string       `json:"sha"`
	Message     string       `json:"message"`
	MessageBody string       `json:"message_body,omitempty"`
	Author      *MinimalUser `json:"author,omitempty"`
	Date        string       `json:"date,omitempty"`
	HTMLURL     string       `json:"html_url,omitempty"`
}

// CommitsComparisonResult represents the result of comparing two git references.
type CommitsComparisonResult struct {
	BaseRef      string           `json:"base_ref"`
	HeadRef      string           `json:"head_ref"`
	Status       string           `json:"status"`
	AheadBy      int              `json:"ahead_by"`
	BehindBy     int              `json:"behind_by"`
	TotalCommits int              `json:"total_commits"`
	Commits      []ChangelogEntry `json:"commits"`
	HTMLURL      string           `json:"html_url,omitempty"`
}

// GetCommitsBetween creates a tool to get commits between two git references (tags, releases, branches, or SHAs).
func GetCommitsBetween(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_commits_between",
			mcp.WithDescription(t("TOOL_GET_COMMITS_BETWEEN_DESCRIPTION", "Get commits between two git references (tags, branches, or SHAs) for changelog generation. Returns normalized commit data including SHA, message, author, and date.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_COMMITS_BETWEEN_USER_TITLE", "Get commits between refs"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("base",
				mcp.Required(),
				mcp.Description("Base reference (tag, branch, or SHA) - the older commit"),
			),
			mcp.WithString("head",
				mcp.Required(),
				mcp.Description("Head reference (tag, branch, or SHA) - the newer commit"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			base, err := RequiredParam[string](request, "base")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			head, err := RequiredParam[string](request, "head")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			comparison, resp, err := client.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to compare commits between %s and %s", base, head),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Convert commits to changelog entries
			entries := make([]ChangelogEntry, 0, len(comparison.Commits))
			for _, commit := range comparison.Commits {
				entry := normalizeCommit(commit)
				entries = append(entries, entry)
			}

			result := CommitsComparisonResult{
				BaseRef:      base,
				HeadRef:      head,
				Status:       comparison.GetStatus(),
				AheadBy:      comparison.GetAheadBy(),
				BehindBy:     comparison.GetBehindBy(),
				TotalCommits: comparison.GetTotalCommits(),
				Commits:      entries,
				HTMLURL:      comparison.GetHTMLURL(),
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// normalizeCommit converts a GitHub commit to a normalized ChangelogEntry.
func normalizeCommit(commit *github.RepositoryCommit) ChangelogEntry {
	entry := ChangelogEntry{
		SHA:     commit.GetSHA(),
		HTMLURL: commit.GetHTMLURL(),
	}

	if commit.Commit != nil {
		message := commit.Commit.GetMessage()
		// Split message into title and body
		if idx := findFirstNewline(message); idx != -1 {
			entry.Message = message[:idx]
			if idx+1 < len(message) {
				entry.MessageBody = trimLeadingNewlines(message[idx+1:])
			}
		} else {
			entry.Message = message
		}

		if commit.Commit.Author != nil && commit.Commit.Author.Date != nil {
			entry.Date = commit.Commit.Author.Date.Format("2006-01-02T15:04:05Z")
		}
	}

	if commit.Author != nil {
		entry.Author = &MinimalUser{
			Login:      commit.Author.GetLogin(),
			ID:         commit.Author.GetID(),
			ProfileURL: commit.Author.GetHTMLURL(),
			AvatarURL:  commit.Author.GetAvatarURL(),
		}
	}

	return entry
}

// findFirstNewline returns the index of the first newline character in the string, or -1 if not found.
func findFirstNewline(s string) int {
	for i, c := range s {
		if c == '\n' {
			return i
		}
	}
	return -1
}

// trimLeadingNewlines removes leading newline characters from a string.
func trimLeadingNewlines(s string) string {
	for len(s) > 0 && (s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	return s
}

// GenerateChangelogPrompt provides a prompt to help the LLM generate changelog content.
func GenerateChangelogPrompt(t translations.TranslationHelperFunc) (mcp.Prompt, server.PromptHandlerFunc) {
	return mcp.NewPrompt("GenerateChangelog",
			mcp.WithPromptDescription(t("PROMPT_GENERATE_CHANGELOG_DESCRIPTION", "Generate a changelog from commits between two git references")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("Repository owner"), mcp.RequiredArgument()),
			mcp.WithArgument("repo", mcp.ArgumentDescription("Repository name"), mcp.RequiredArgument()),
			mcp.WithArgument("base", mcp.ArgumentDescription("Base reference (tag, branch, or SHA) - the older commit"), mcp.RequiredArgument()),
			mcp.WithArgument("head", mcp.ArgumentDescription("Head reference (tag, branch, or SHA) - the newer commit"), mcp.RequiredArgument()),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			owner := request.Params.Arguments["owner"]
			repo := request.Params.Arguments["repo"]
			base := request.Params.Arguments["base"]
			head := request.Params.Arguments["head"]

			messages := []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent("You are an expert technical writer specializing in generating clear, well-organized changelogs for software projects. Your task is to analyze commit data and produce a professional changelog following conventional changelog standards."),
				},
				{
					Role: "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I need you to generate a changelog for %s/%s comparing changes from %s to %s.\n\nPlease:\n1. First use the `get_commits_between` tool to fetch the commits\n2. Analyze and categorize each commit by type (features, bug fixes, breaking changes, etc.)\n3. Generate a well-formatted changelog with:\n   - Version/release header\n   - Categorized sections for different types of changes\n   - Each entry should include a brief description and link to the commit\n   - Highlight any breaking changes prominently",
						owner, repo, base, head)),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("I'll help you generate a changelog for %s/%s by comparing %s to %s. Let me start by fetching the commits between these references.", owner, repo, base, head)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Please format the changelog following the Keep a Changelog format (https://keepachangelog.com). Group changes into these categories when applicable:\n- Added: for new features\n- Changed: for changes in existing functionality\n- Deprecated: for soon-to-be removed features\n- Removed: for removed features\n- Fixed: for bug fixes\n- Security: for vulnerability fixes"),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}

// GetChangelogResourceContent defines the resource template and handler for accessing CHANGELOG.md content.
func GetChangelogResourceContent(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
			"changelog://{owner}/{repo}", // Resource template
			t("RESOURCE_CHANGELOG_DESCRIPTION", "Changelog content for a repository"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			// Extract owner and repo from arguments
			o, ok := request.Params.Arguments["owner"].([]string)
			if !ok || len(o) == 0 {
				return nil, fmt.Errorf("owner is required")
			}
			owner := o[0]

			r, ok := request.Params.Arguments["repo"].([]string)
			if !ok || len(r) == 0 {
				return nil, fmt.Errorf("repo is required")
			}
			repo := r[0]

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Try common changelog file names
			changelogFiles := []string{"CHANGELOG.md", "Changelog.md", "changelog.md", "CHANGELOG", "HISTORY.md", "CHANGES.md"}
			var content *github.RepositoryContent

			for _, filename := range changelogFiles {
				fileContent, _, resp, err := client.Repositories.GetContents(ctx, owner, repo, filename, nil)
				if err == nil && resp.StatusCode == 200 && fileContent != nil {
					content = fileContent
					break
				}
			}

			if content == nil {
				return nil, fmt.Errorf("no changelog file found in repository %s/%s", owner, repo)
			}

			// Decode content
			decoded, err := content.GetContent()
			if err != nil {
				return nil, fmt.Errorf("failed to decode changelog content: %w", err)
			}

			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      request.Params.URI,
					MIMEType: "text/markdown",
					Text:     decoded,
				},
			}, nil
		}
}
