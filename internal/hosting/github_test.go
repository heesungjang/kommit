package hosting

import (
	"testing"
)

func TestRepoRefFromRemoteURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    RepoRef
		wantErr bool
	}{
		{
			name: "HTTPS with .git",
			url:  "https://github.com/heesungjang/kommit.git",
			want: RepoRef{Owner: "heesungjang", Repo: "kommit"},
		},
		{
			name: "HTTPS without .git",
			url:  "https://github.com/heesungjang/kommit",
			want: RepoRef{Owner: "heesungjang", Repo: "kommit"},
		},
		{
			name: "SSH format",
			url:  "git@github.com:heesungjang/kommit.git",
			want: RepoRef{Owner: "heesungjang", Repo: "kommit"},
		},
		{
			name: "SSH without .git",
			url:  "git@github.com:heesungjang/kommit",
			want: RepoRef{Owner: "heesungjang", Repo: "kommit"},
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name: "GitLab HTTPS",
			url:  "https://gitlab.com/group/project.git",
			want: RepoRef{Owner: "group", Repo: "project"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RepoRefFromRemoteURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RepoRefFromRemoteURL(%q) error = %v, wantErr = %v", tt.url, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Owner != tt.want.Owner || got.Repo != tt.want.Repo {
				t.Errorf("RepoRefFromRemoteURL(%q) = %+v, want %+v", tt.url, got, tt.want)
			}
		})
	}
}

func TestPullRequestStatusIcon(t *testing.T) {
	tests := []struct {
		name string
		pr   PullRequest
		want string
	}{
		{name: "open", pr: PullRequest{State: "open"}, want: "●"},
		{name: "draft", pr: PullRequest{State: "open", Draft: true}, want: "◌"},
		{name: "closed", pr: PullRequest{State: "closed"}, want: "✕"},
		{name: "merged", pr: PullRequest{State: "merged"}, want: "◆"},
		{name: "unknown", pr: PullRequest{State: "unknown"}, want: "○"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pr.StatusIcon()
			if got != tt.want {
				t.Errorf("StatusIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}
