package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNextTier(t *testing.T) {
	tiers := []int{2, 8, 16, 32}
	tests := []struct {
		count int
		want  string
	}{
		{0, "needs 2"},
		{1, "needs 2"},
		{2, "next tier at 8"},
		{3, "next tier at 8"},
		{8, "next tier at 16"},
		{31, "next tier at 32"},
		{32, "top tier reached"},
		{100, "top tier reached"},
	}
	for _, tt := range tests {
		if got := nextTier(tt.count, tiers); got != tt.want {
			t.Errorf("nextTier(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestFetchProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		var req struct {
			Variables map[string]string `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if got := req.Variables["prs"]; got != "author:octo is:pr is:merged" {
			t.Errorf("prs = %q", got)
		}
		w.Write([]byte(`{"data":{
			"user":{
				"repositoryDiscussionComments":{"totalCount":3},
				"repositories":{"nodes":[{"nameWithOwner":"octo/top","stargazerCount":6}]}
			},
			"search":{"issueCount":12}
		}}`))
	}))
	defer server.Close()

	got, err := fetchProgress(context.Background(), server.Client(), server.URL, "test-token", "octo")
	if err != nil {
		t.Fatal(err)
	}
	want := Progress{AcceptedAnswers: 3, MergedPRs: 12, TopRepo: "octo/top", TopRepoStars: 6}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFetchProgressErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"http error", http.StatusUnauthorized, `{}`, "401"},
		{"graphql error", http.StatusOK, `{"errors":[{"message":"rate limited"}]}`, "rate limited"},
		{"unknown user", http.StatusOK, `{"data":{"user":null,"search":{"issueCount":0}}}`, "not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			_, err := fetchProgress(context.Background(), server.Client(), server.URL, "t", "octo")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestReport(t *testing.T) {
	var out bytes.Buffer
	if err := report(&out, Progress{AcceptedAnswers: 3, MergedPRs: 1, TopRepoStars: 6}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"3 accepted answers", "next tier at 8", "1 merged PRs", "needs 2", "needs 16"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunRequiresOneArgument(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Error("expected a usage error")
	}
}
