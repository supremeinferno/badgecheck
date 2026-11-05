// Command badgecheck shows a GitHub user's progress toward achievements that
// can be counted through the GitHub GraphQL API.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"
)

const defaultEndpoint = "https://api.github.com/graphql"

const progressQuery = `query($login: String!, $prs: String!) {
  user(login: $login) {
    repositoryDiscussionComments(onlyAnswers: true) { totalCount }
    repositories(first: 1, ownerAffiliations: OWNER, isFork: false,
                 orderBy: {field: STARGAZERS, direction: DESC}) {
      nodes { nameWithOwner stargazerCount }
    }
  }
  search(type: ISSUE, query: $prs, first: 1) { issueCount }
}`

// Progress holds the counts used to work out achievement tiers.
type Progress struct {
	AcceptedAnswers int
	MergedPRs       int
	TopRepo         string
	TopRepoStars    int
}

// Badge describes one achievement and the count each of its tiers needs.
type Badge struct {
	Name  string
	Unit  string
	Tiers []int
	Count func(Progress) int
}

var badges = []Badge{
	{"Galaxy Brain", "accepted answers", []int{2, 8, 16, 32}, func(p Progress) int { return p.AcceptedAnswers }},
	{"Pull Shark", "merged PRs", []int{2, 16, 128, 1024}, func(p Progress) int { return p.MergedPRs }},
	{"Starstruck", "stars on top repo", []int{16, 128, 512, 4096}, func(p Progress) int { return p.TopRepoStars }},
}

// nextTier returns the status text for a count against ascending tiers.
func nextTier(count int, tiers []int) string {
	for i, tier := range tiers {
		if count < tier {
			if i == 0 {
				return fmt.Sprintf("needs %d", tier)
			}
			return fmt.Sprintf("next tier at %d", tier)
		}
	}
	return "top tier reached"
}

// resolveToken reads GITHUB_TOKEN, then falls back to the GitHub CLI.
func resolveToken(ctx context.Context) (string, error) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		return "", errors.New("no token found: set GITHUB_TOKEN or run `gh auth login`")
	}
	return strings.TrimSpace(string(out)), nil
}

// fetchProgress asks the GraphQL API for a user's counts.
func fetchProgress(ctx context.Context, client *http.Client, endpoint, token, login string) (Progress, error) {
	body, err := json.Marshal(map[string]any{
		"query": progressQuery,
		"variables": map[string]string{
			"login": login,
			"prs":   fmt.Sprintf("author:%s is:pr is:merged", login),
		},
	})
	if err != nil {
		return Progress{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Progress{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "badgecheck")

	resp, err := client.Do(req)
	if err != nil {
		return Progress{}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Progress{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Progress{}, fmt.Errorf("github api returned %s", resp.Status)
	}

	var result struct {
		Data struct {
			User *struct {
				RepositoryDiscussionComments struct {
					TotalCount int `json:"totalCount"`
				} `json:"repositoryDiscussionComments"`
				Repositories struct {
					Nodes []struct {
						NameWithOwner  string `json:"nameWithOwner"`
						StargazerCount int    `json:"stargazerCount"`
					} `json:"nodes"`
				} `json:"repositories"`
			} `json:"user"`
			Search struct {
				IssueCount int `json:"issueCount"`
			} `json:"search"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return Progress{}, err
	}
	if len(result.Errors) > 0 {
		return Progress{}, errors.New(result.Errors[0].Message)
	}
	if result.Data.User == nil {
		return Progress{}, fmt.Errorf("user %q not found", login)
	}

	progress := Progress{
		AcceptedAnswers: result.Data.User.RepositoryDiscussionComments.TotalCount,
		MergedPRs:       result.Data.Search.IssueCount,
	}
	if nodes := result.Data.User.Repositories.Nodes; len(nodes) > 0 {
		progress.TopRepo = nodes[0].NameWithOwner
		progress.TopRepoStars = nodes[0].StargazerCount
	}
	return progress, nil
}

// report writes one line per badge.
func report(w io.Writer, p Progress) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, badge := range badges {
		count := badge.Count(p)
		fmt.Fprintf(tw, "%s\t%d %s\t%s\n", badge.Name, count, badge.Unit, nextTier(count, badge.Tiers))
	}
	return tw.Flush()
}

func run(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: badgecheck <github-username>")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := resolveToken(ctx)
	if err != nil {
		return err
	}
	progress, err := fetchProgress(ctx, http.DefaultClient, defaultEndpoint, token, args[0])
	if err != nil {
		return err
	}
	return report(stdout, progress)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "badgecheck:", err)
		os.Exit(1)
	}
}
