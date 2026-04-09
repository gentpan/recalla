package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client GitHub API 客户端
type Client struct {
	token  string
	client *http.Client
}

// NewClient 创建 GitHub 客户端
func NewClient(token string) *Client {
	return &Client{
		token:  token,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Repo 仓库信息
type Repo struct {
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	Private     bool      `json:"private"`
	DefaultBranch string  `json:"default_branch"`
	Language    string    `json:"language"`
	UpdatedAt   time.Time `json:"updated_at"`
	HTMLURL     string    `json:"html_url"`
}

// Commit 提交信息
type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	URL     string `json:"url"`
}

// ListRepos 获取用户仓库列表
func (c *Client) ListRepos(ctx context.Context) ([]Repo, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/repos?sort=updated&per_page=50", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API 错误 (%d): %s", resp.StatusCode, string(b))
	}

	var repos []Repo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("解析仓库列表失败: %w", err)
	}
	return repos, nil
}

// ListCommits 获取仓库最近的提交
func (c *Client) ListCommits(ctx context.Context, owner, repo string, limit int) ([]Commit, error) {
	if limit <= 0 {
		limit = 20
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?per_page=%d", owner, repo, limit)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API 错误 (%d): %s", resp.StatusCode, string(b))
	}

	var raw []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析提交列表失败: %w", err)
	}

	var commits []Commit
	for _, r := range raw {
		commits = append(commits, Commit{
			SHA:     r.SHA[:7],
			Message: r.Commit.Message,
			Author:  r.Commit.Author.Name,
			Date:    r.Commit.Author.Date,
			URL:     r.HTMLURL,
		})
	}
	return commits, nil
}

// WebhookEvent Webhook 事件
type WebhookEvent struct {
	Type    string // push, pull_request
	Repo    string
	Branch  string
	Commits []WebhookCommit
	PR      *WebhookPR
}

// WebhookCommit Webhook 提交
type WebhookCommit struct {
	SHA     string
	Message string
	Author  string
	Added   []string
	Modified []string
	Removed  []string
}

// WebhookPR Webhook PR
type WebhookPR struct {
	Number int
	Title  string
	Body   string
	State  string
	Action string
	Merged bool
	Author string
}

// ParseWebhook 解析 GitHub Webhook 请求
func ParseWebhook(r *http.Request, secret string) (*WebhookEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("读取请求体失败: %w", err)
	}

	// 验证签名
	if secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if sig == "" {
			return nil, fmt.Errorf("缺少签名")
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(sig), []byte(expected)) {
			return nil, fmt.Errorf("签名验证失败")
		}
	}

	eventType := r.Header.Get("X-GitHub-Event")
	event := &WebhookEvent{Type: eventType}

	switch eventType {
	case "push":
		var payload struct {
			Ref        string `json:"ref"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
			Commits []struct {
				ID      string   `json:"id"`
				Message string   `json:"message"`
				Author  struct {
					Name string `json:"name"`
				} `json:"author"`
				Added    []string `json:"added"`
				Modified []string `json:"modified"`
				Removed  []string `json:"removed"`
			} `json:"commits"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("解析 push 事件失败: %w", err)
		}
		event.Repo = payload.Repository.FullName
		// refs/heads/main -> main
		if len(payload.Ref) > 11 {
			event.Branch = payload.Ref[11:]
		}
		for _, c := range payload.Commits {
			sha := c.ID
			if len(sha) > 7 {
				sha = sha[:7]
			}
			event.Commits = append(event.Commits, WebhookCommit{
				SHA: sha, Message: c.Message, Author: c.Author.Name,
				Added: c.Added, Modified: c.Modified, Removed: c.Removed,
			})
		}

	case "pull_request":
		var payload struct {
			Action      string `json:"action"`
			PullRequest struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
				Body   string `json:"body"`
				State  string `json:"state"`
				Merged bool   `json:"merged"`
				User   struct {
					Login string `json:"login"`
				} `json:"user"`
			} `json:"pull_request"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("解析 PR 事件失败: %w", err)
		}
		event.Repo = payload.Repository.FullName
		event.PR = &WebhookPR{
			Number: payload.PullRequest.Number,
			Title:  payload.PullRequest.Title,
			Body:   payload.PullRequest.Body,
			State:  payload.PullRequest.State,
			Action: payload.Action,
			Merged: payload.PullRequest.Merged,
			Author: payload.PullRequest.User.Login,
		}

	default:
		return nil, fmt.Errorf("不支持的事件类型: %s", eventType)
	}

	return event, nil
}
