package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetCommitsBetween(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := GetCommitsBetween(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_commits_between", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "base")
	assert.Contains(t, tool.InputSchema.Properties, "head")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "base", "head"})

	// Verify tool annotations
	assert.NotNil(t, tool.Annotations.ReadOnlyHint)
	assert.True(t, *tool.Annotations.ReadOnlyHint)

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult *CommitsComparisonResult
		expectedErrMsg string
	}{
		{
			name: "successful comparison with commits",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposCompareByOwnerByRepoByBasehead,
					github.CommitsComparison{
						Status:       github.Ptr("ahead"),
						AheadBy:      github.Ptr(2),
						BehindBy:     github.Ptr(0),
						TotalCommits: github.Ptr(2),
						HTMLURL:      github.Ptr("https://github.com/owner/repo/compare/v1.0.0...v1.1.0"),
						Commits: []*github.RepositoryCommit{
							{
								SHA:     github.Ptr("abc123"),
								HTMLURL: github.Ptr("https://github.com/owner/repo/commit/abc123"),
								Commit: &github.Commit{
									Message: github.Ptr("feat: add new feature"),
									Author: &github.CommitAuthor{
										Name:  github.Ptr("Test User"),
										Email: github.Ptr("test@example.com"),
										Date:  &github.Timestamp{Time: testTime},
									},
								},
								Author: &github.User{
									Login:   github.Ptr("testuser"),
									ID:      github.Ptr(int64(123)),
									HTMLURL: github.Ptr("https://github.com/testuser"),
								},
							},
							{
								SHA:     github.Ptr("def456"),
								HTMLURL: github.Ptr("https://github.com/owner/repo/commit/def456"),
								Commit: &github.Commit{
									Message: github.Ptr("fix: resolve bug\n\nThis fixes the issue with parsing."),
									Author: &github.CommitAuthor{
										Name:  github.Ptr("Another User"),
										Email: github.Ptr("another@example.com"),
										Date:  &github.Timestamp{Time: testTime.Add(1 * time.Hour)},
									},
								},
								Author: &github.User{
									Login:   github.Ptr("anotheruser"),
									ID:      github.Ptr(int64(456)),
									HTMLURL: github.Ptr("https://github.com/anotheruser"),
								},
							},
						},
					},
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"base":  "v1.0.0",
				"head":  "v1.1.0",
			},
			expectError: false,
			expectedResult: &CommitsComparisonResult{
				BaseRef:      "v1.0.0",
				HeadRef:      "v1.1.0",
				Status:       "ahead",
				AheadBy:      2,
				BehindBy:     0,
				TotalCommits: 2,
				HTMLURL:      "https://github.com/owner/repo/compare/v1.0.0...v1.1.0",
				Commits: []ChangelogEntry{
					{
						SHA:     "abc123",
						Message: "feat: add new feature",
						HTMLURL: "https://github.com/owner/repo/commit/abc123",
						Date:    "2024-01-15T10:30:00Z",
						Author: &MinimalUser{
							Login:      "testuser",
							ID:         123,
							ProfileURL: "https://github.com/testuser",
						},
					},
					{
						SHA:         "def456",
						Message:     "fix: resolve bug",
						MessageBody: "This fixes the issue with parsing.",
						HTMLURL:     "https://github.com/owner/repo/commit/def456",
						Date:        "2024-01-15T11:30:00Z",
						Author: &MinimalUser{
							Login:      "anotheruser",
							ID:         456,
							ProfileURL: "https://github.com/anotheruser",
						},
					},
				},
			},
		},
		{
			name: "comparison with no commits (identical refs)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposCompareByOwnerByRepoByBasehead,
					github.CommitsComparison{
						Status:       github.Ptr("identical"),
						AheadBy:      github.Ptr(0),
						BehindBy:     github.Ptr(0),
						TotalCommits: github.Ptr(0),
						HTMLURL:      github.Ptr("https://github.com/owner/repo/compare/v1.0.0...v1.0.0"),
						Commits:      []*github.RepositoryCommit{},
					},
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"base":  "v1.0.0",
				"head":  "v1.0.0",
			},
			expectError: false,
			expectedResult: &CommitsComparisonResult{
				BaseRef:      "v1.0.0",
				HeadRef:      "v1.0.0",
				Status:       "identical",
				AheadBy:      0,
				BehindBy:     0,
				TotalCommits: 0,
				HTMLURL:      "https://github.com/owner/repo/compare/v1.0.0...v1.0.0",
				Commits:      []ChangelogEntry{},
			},
		},
		{
			name:         "missing required owner parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"repo": "repo",
				"base": "v1.0.0",
				"head": "v1.1.0",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required base parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"head":  "v1.1.0",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: base",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := GetCommitsBetween(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.Nil(t, err)
				require.True(t, result.IsError)
				textContent := getErrorResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.Nil(t, err)
			require.False(t, result.IsError)

			// Parse the result
			var comparison CommitsComparisonResult
			err = json.Unmarshal([]byte(getTextResult(t, result).Text), &comparison)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedResult.BaseRef, comparison.BaseRef)
			assert.Equal(t, tc.expectedResult.HeadRef, comparison.HeadRef)
			assert.Equal(t, tc.expectedResult.Status, comparison.Status)
			assert.Equal(t, tc.expectedResult.AheadBy, comparison.AheadBy)
			assert.Equal(t, tc.expectedResult.BehindBy, comparison.BehindBy)
			assert.Equal(t, tc.expectedResult.TotalCommits, comparison.TotalCommits)
			assert.Equal(t, tc.expectedResult.HTMLURL, comparison.HTMLURL)
			assert.Len(t, comparison.Commits, len(tc.expectedResult.Commits))

			for i, expected := range tc.expectedResult.Commits {
				actual := comparison.Commits[i]
				assert.Equal(t, expected.SHA, actual.SHA)
				assert.Equal(t, expected.Message, actual.Message)
				assert.Equal(t, expected.MessageBody, actual.MessageBody)
				assert.Equal(t, expected.HTMLURL, actual.HTMLURL)
				assert.Equal(t, expected.Date, actual.Date)
				if expected.Author != nil {
					require.NotNil(t, actual.Author)
					assert.Equal(t, expected.Author.Login, actual.Author.Login)
					assert.Equal(t, expected.Author.ID, actual.Author.ID)
					assert.Equal(t, expected.Author.ProfileURL, actual.Author.ProfileURL)
				}
			}
		})
	}
}

func Test_normalizeCommit(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		commit   *github.RepositoryCommit
		expected ChangelogEntry
	}{
		{
			name: "commit with title only",
			commit: &github.RepositoryCommit{
				SHA:     github.Ptr("abc123"),
				HTMLURL: github.Ptr("https://github.com/owner/repo/commit/abc123"),
				Commit: &github.Commit{
					Message: github.Ptr("feat: add feature"),
					Author: &github.CommitAuthor{
						Date: &github.Timestamp{Time: testTime},
					},
				},
				Author: &github.User{
					Login: github.Ptr("testuser"),
					ID:    github.Ptr(int64(123)),
				},
			},
			expected: ChangelogEntry{
				SHA:     "abc123",
				Message: "feat: add feature",
				HTMLURL: "https://github.com/owner/repo/commit/abc123",
				Date:    "2024-01-15T10:30:00Z",
				Author: &MinimalUser{
					Login: "testuser",
					ID:    123,
				},
			},
		},
		{
			name: "commit with title and body",
			commit: &github.RepositoryCommit{
				SHA:     github.Ptr("def456"),
				HTMLURL: github.Ptr("https://github.com/owner/repo/commit/def456"),
				Commit: &github.Commit{
					Message: github.Ptr("fix: resolve bug\n\nThis is the body explaining the fix."),
					Author: &github.CommitAuthor{
						Date: &github.Timestamp{Time: testTime},
					},
				},
			},
			expected: ChangelogEntry{
				SHA:         "def456",
				Message:     "fix: resolve bug",
				MessageBody: "This is the body explaining the fix.",
				HTMLURL:     "https://github.com/owner/repo/commit/def456",
				Date:        "2024-01-15T10:30:00Z",
			},
		},
		{
			name: "commit with multiple newlines between title and body",
			commit: &github.RepositoryCommit{
				SHA:     github.Ptr("ghi789"),
				HTMLURL: github.Ptr("https://github.com/owner/repo/commit/ghi789"),
				Commit: &github.Commit{
					Message: github.Ptr("chore: update deps\n\n\n\nMultiple blank lines before this."),
					Author: &github.CommitAuthor{
						Date: &github.Timestamp{Time: testTime},
					},
				},
			},
			expected: ChangelogEntry{
				SHA:         "ghi789",
				Message:     "chore: update deps",
				MessageBody: "Multiple blank lines before this.",
				HTMLURL:     "https://github.com/owner/repo/commit/ghi789",
				Date:        "2024-01-15T10:30:00Z",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeCommit(tc.commit)
			assert.Equal(t, tc.expected.SHA, result.SHA)
			assert.Equal(t, tc.expected.Message, result.Message)
			assert.Equal(t, tc.expected.MessageBody, result.MessageBody)
			assert.Equal(t, tc.expected.HTMLURL, result.HTMLURL)
			assert.Equal(t, tc.expected.Date, result.Date)
			if tc.expected.Author != nil {
				require.NotNil(t, result.Author)
				assert.Equal(t, tc.expected.Author.Login, result.Author.Login)
				assert.Equal(t, tc.expected.Author.ID, result.Author.ID)
			}
		})
	}
}

func Test_GenerateChangelogPrompt(t *testing.T) {
	prompt, handler := GenerateChangelogPrompt(translations.NullTranslationHelper)

	assert.Equal(t, "GenerateChangelog", prompt.Name)
	assert.NotEmpty(t, prompt.Description)

	// Test the prompt handler
	request := mcp.GetPromptRequest{}
	request.Params.Name = "GenerateChangelog"
	request.Params.Arguments = map[string]string{
		"owner": "testowner",
		"repo":  "testrepo",
		"base":  "v1.0.0",
		"head":  "v2.0.0",
	}

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Messages)

	// Verify messages contain expected content
	var foundInstructions, foundRepoDetails bool
	for _, msg := range result.Messages {
		if textContent, ok := msg.Content.(mcp.TextContent); ok {
			if textContent.Text != "" {
				if containsAll(textContent.Text, []string{"changelog", "testowner", "testrepo"}) {
					foundRepoDetails = true
				}
				if containsAll(textContent.Text, []string{"Keep a Changelog", "Added", "Changed", "Fixed"}) {
					foundInstructions = true
				}
			}
		}
	}
	assert.True(t, foundRepoDetails, "Prompt should contain repository details")
	assert.True(t, foundInstructions, "Prompt should contain changelog format instructions")
}

func containsAll(s string, substrings []string) bool {
	for _, sub := range substrings {
		if !containsString(s, sub) {
			return false
		}
	}
	return true
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
