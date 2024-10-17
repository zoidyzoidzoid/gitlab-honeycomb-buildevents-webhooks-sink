package types

import (
	"encoding/json"
	"strings"
	"time"
)

// Commit contains all of the GitLab commit information.
type Commit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Title     string    `json:"title"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
	Author    Author    `json:"author"`
	Added     []string  `json:"added"`
	Modified  []string  `json:"modified"`
	Removed   []string  `json:"removed"`
}

// Author contains all of the GitLab author information.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Project contains all of the GitLab project information.
type Project struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	WebURL            string `json:"web_url"`
	AvatarURL         string `json:"avatar_url"`
	GitSSHURL         string `json:"git_ssh_url"`
	GitHTTPURL        string `json:"git_http_url"`
	Namespace         string `json:"namespace"`
	VisibilityLevel   int64  `json:"visibility_level"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
	Homepage          string `json:"homepage"`
	URL               string `json:"url"`
	SSHURL            string `json:"ssh_url"`
	HTTPURL           string `json:"http_url"`
}

// Repository contains all of the GitLab repository information.
type Repository struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	Description     string `json:"description"`
	Homepage        string `json:"homepage"`
	GitSSHURL       string `json:"git_ssh_url"`
	GitHTTPURL      string `json:"git_http_url"`
	VisibilityLevel int64  `json:"visibility_level"`
}

// User contains all of the GitLab user information.
type User struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	UserName  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

// Runner represents a runner agent.
type Runner struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	IsShared    bool   `json:"is_shared"`
}

type GitLabTimestamp time.Time

func (timestamp *GitLabTimestamp) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		return nil
	}
	// 2022-10-17 14:44:20 +1300 -> GitLab Timestamp Format
	// 2006-01-02T15:04:05Z07:00 -> Go Str Pattern
	t, err := time.Parse("2006-01-02 15:04:05 -0700", s)
	if err != nil {
		return err
	}
	*timestamp = GitLabTimestamp(t)
	return nil
}

func (timestamp *GitLabTimestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(*timestamp))
}
