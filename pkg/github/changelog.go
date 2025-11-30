package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GenerateChangelogPrompt provides a guided prompt for generating or updating a CHANGELOG.md file.
// This prompt instructs the LLM to use existing tools (list_commits, list_pull_requests, list_releases, etc.)
// to gather information about changes, then write appropriate changelog entries.
func GenerateChangelogPrompt(t translations.TranslationHelperFunc) (mcp.Prompt, server.PromptHandlerFunc) {
	return mcp.NewPrompt("generate-changelog",
			mcp.WithPromptDescription(t("PROMPT_GENERATE_CHANGELOG_DESCRIPTION", "Generate or update a CHANGELOG.md file based on recent repository changes")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("Repository owner"), mcp.RequiredArgument()),
			mcp.WithArgument("repo", mcp.ArgumentDescription("Repository name"), mcp.RequiredArgument()),
			mcp.WithArgument("since_version", mcp.ArgumentDescription("Previous version tag to compare from (e.g., 'v1.0.0'). If not provided, uses the latest release.")),
			mcp.WithArgument("new_version", mcp.ArgumentDescription("New version being released (e.g., 'v1.1.0'). If not provided, will suggest based on changes.")),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			owner := request.Params.Arguments["owner"]
			repo := request.Params.Arguments["repo"]

			sinceVersion := ""
			if v, exists := request.Params.Arguments["since_version"]; exists {
				sinceVersion = fmt.Sprintf("%v", v)
			}

			newVersion := ""
			if v, exists := request.Params.Arguments["new_version"]; exists {
				newVersion = fmt.Sprintf("%v", v)
			}

			contextInfo := fmt.Sprintf("Repository: %s/%s", owner, repo)
			if sinceVersion != "" {
				contextInfo += fmt.Sprintf("\nComparing changes since: %s", sinceVersion)
			}
			if newVersion != "" {
				contextInfo += fmt.Sprintf("\nNew version: %s", newVersion)
			}

			messages := []mcp.PromptMessage{
				{
					Role: "user",
					Content: mcp.NewTextContent(`You are a changelog generation assistant. Your task is to create or update a CHANGELOG.md file for a GitHub repository following the Keep a Changelog format (https://keepachangelog.com/).

Your workflow:
1. Use existing tools to gather information about changes:
   - Use 'get_latest_release' to find the most recent release
   - Use 'list_releases' to see release history
   - Use 'list_commits' to see commits since the last release
   - Use 'list_pull_requests' to find merged PRs (use state: "closed")
   - Use 'get_file_contents' to read the current CHANGELOG.md if it exists

2. Categorize changes using Keep a Changelog categories:
   - Added: New features
   - Changed: Changes in existing functionality  
   - Deprecated: Features that will be removed
   - Removed: Removed features
   - Fixed: Bug fixes
   - Security: Security-related changes

3. Write clear, user-focused changelog entries that explain the impact of each change.

4. Use 'create_or_update_file' to update CHANGELOG.md with the new entries.

Important guidelines:
- Write entries from the user's perspective, not the developer's
- Group related changes together
- Include PR/issue references where applicable
- Follow semantic versioning when suggesting version numbers`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent(contextInfo),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("I'll help you generate a changelog for %s/%s. Let me start by gathering information about recent changes using the available tools.", owner, repo)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Please proceed with gathering the information and generating the changelog. Start by checking the latest release and then review the commits and pull requests since that release."),
				},
			}

			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
