package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateChangelogPrompt(t *testing.T) {
	stubTranslation := func(_, defaultValue string) string {
		return defaultValue
	}

	prompt, handler := GenerateChangelogPrompt(translations.TranslationHelperFunc(stubTranslation))

	t.Run("prompt definition", func(t *testing.T) {
		assert.Equal(t, "generate-changelog", prompt.Name)
		assert.Contains(t, prompt.Description, "Generate or update a CHANGELOG.md")
		require.NotNil(t, prompt.Arguments)
		assert.Len(t, prompt.Arguments, 4)

		// Check required arguments
		var ownerArg, repoArg, sinceVersionArg, newVersionArg *mcp.PromptArgument
		for _, arg := range prompt.Arguments {
			switch arg.Name {
			case "owner":
				ownerArg = &arg
			case "repo":
				repoArg = &arg
			case "since_version":
				sinceVersionArg = &arg
			case "new_version":
				newVersionArg = &arg
			}
		}

		require.NotNil(t, ownerArg)
		assert.True(t, ownerArg.Required)

		require.NotNil(t, repoArg)
		assert.True(t, repoArg.Required)

		require.NotNil(t, sinceVersionArg)
		assert.False(t, sinceVersionArg.Required)

		require.NotNil(t, newVersionArg)
		assert.False(t, newVersionArg.Required)
	})

	t.Run("handler with required arguments only", func(t *testing.T) {
		request := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name: "generate-changelog",
				Arguments: map[string]string{
					"owner": "octocat",
					"repo":  "hello-world",
				},
			},
		}

		result, err := handler(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.Messages)

		// Check that messages contain expected content
		hasKeepChangelogReference := false
		hasToolReferences := false
		for _, msg := range result.Messages {
			content, ok := msg.Content.(mcp.TextContent)
			if ok {
				if contains(content.Text, "Keep a Changelog") {
					hasKeepChangelogReference = true
				}
				if contains(content.Text, "list_commits") || contains(content.Text, "list_releases") {
					hasToolReferences = true
				}
			}
		}
		assert.True(t, hasKeepChangelogReference, "prompt should reference Keep a Changelog format")
		assert.True(t, hasToolReferences, "prompt should reference existing tools for gathering changes")
	})

	t.Run("handler with all arguments", func(t *testing.T) {
		request := mcp.GetPromptRequest{
			Params: mcp.GetPromptParams{
				Name: "generate-changelog",
				Arguments: map[string]string{
					"owner":         "octocat",
					"repo":          "hello-world",
					"since_version": "v1.0.0",
					"new_version":   "v1.1.0",
				},
			},
		}

		result, err := handler(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.Messages)

		// Check that context info includes version information
		hasVersionInfo := false
		for _, msg := range result.Messages {
			content, ok := msg.Content.(mcp.TextContent)
			if ok {
				if contains(content.Text, "v1.0.0") && contains(content.Text, "v1.1.0") {
					hasVersionInfo = true
					break
				}
			}
		}
		assert.True(t, hasVersionInfo, "prompt should include version information when provided")
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
