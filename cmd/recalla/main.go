package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

var (
	serverURL = envOr("RECALLA_URL", "https://peter.recalla.dev")
	apiKey    = envOr("RECALLA_KEY", "")
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "search", "s":
		if len(os.Args) < 3 { fmt.Println("Usage: recalla search <query>"); return }
		query := strings.Join(os.Args[2:], " ")
		doSearch(query)

	case "save":
		if len(os.Args) < 3 { fmt.Println("Usage: recalla save <content>"); return }
		content := strings.Join(os.Args[2:], " ")
		doSave(content)

	case "context", "c":
		project := ""
		if len(os.Args) >= 3 { project = os.Args[2] }
		doContext(project)

	case "projects":
		doProjects()

	case "briefing":
		project := ""
		if len(os.Args) >= 3 { project = os.Args[2] }
		doBriefing(project)

	case "status":
		doStatus()

	case "push":
		if len(os.Args) < 4 { fmt.Println("Usage: recalla push <file-key> <file-path>"); return }
		doPush(os.Args[2], os.Args[3])

	case "pull":
		if len(os.Args) < 3 { fmt.Println("Usage: recalla pull <file-key>"); return }
		doPull(os.Args[2])

	case "help", "-h", "--help":
		printHelp()

	default:
		fmt.Printf("Unknown command: %s\nRun 'recalla help' for usage.\n", cmd)
	}
}

func doSearch(query string) {
	data := apiPost("/api/memory/search", map[string]any{"query": query, "limit": 10})
	results, _ := data["results"].([]any)
	if len(results) == 0 {
		fmt.Println("No memories found.")
		return
	}
	for i, r := range results {
		m, _ := r.(map[string]any)
		content := fmt.Sprintf("%v", m["content"])
		if len(content) > 120 { content = content[:120] + "..." }
		score, _ := m["score"].(float64)
		fmt.Printf("%d. [%s] %s\n   %s\n   Score: %.0f%% | %s\n\n",
			i+1, m["type"], m["project"], content, score*100, m["created_at"])
	}
}

func doSave(content string) {
	// 从当前目录名推断项目
	dir, _ := os.Getwd()
	parts := strings.Split(dir, "/")
	project := parts[len(parts)-1]

	data := apiPost("/api/memory/save", map[string]any{
		"project": project, "type": "note", "content": content, "tags": []string{"cli"},
	})
	if id, ok := data["id"].(string); ok {
		fmt.Printf("Saved to %s [note] ID: %s\n", project, id[:8])
	}
}

func doContext(project string) {
	if project == "" {
		dir, _ := os.Getwd()
		parts := strings.Split(dir, "/")
		project = parts[len(parts)-1]
	}
	data := apiPost("/api/context/restore", map[string]any{"project": project})
	fmt.Printf("Project: %s\n", project)
	if d, ok := data["last_device"].(string); ok && d != "" { fmt.Printf("Device:  %s\n", d) }
	if b, ok := data["last_branch"].(string); ok && b != "" { fmt.Printf("Branch:  %s\n", b) }
	memories, _ := data["recent_memories"].([]any)
	fmt.Printf("\nRecent memories: %d\n", len(memories))
	for i, r := range memories {
		if i >= 5 { break }
		m, _ := r.(map[string]any)
		content := fmt.Sprintf("%v", m["content"])
		if len(content) > 80 { content = content[:80] + "..." }
		fmt.Printf("  - [%s] %s\n", m["type"], content)
	}
}

func doProjects() {
	data := apiGet("/api/projects")
	projects, _ := data["projects"].([]any)
	if len(projects) == 0 { fmt.Println("No projects."); return }
	for _, p := range projects {
		fmt.Printf("  %s\n", p)
	}
}

func doBriefing(project string) {
	data := apiPost("/api/briefing", map[string]any{"project": project})
	if content, ok := data["content"].(string); ok {
		fmt.Println(content)
	}
}

func doStatus() {
	data := apiGet("/api/stats")
	fmt.Printf("Memories:  %v\n", data["memories"])
	fmt.Printf("Projects:  %v\n", data["projects"])
	fmt.Printf("Sessions:  %v\n", data["sessions"])
}

func doPush(fileKey, filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil { fmt.Printf("Error reading file: %v\n", err); return }
	hostname, _ := os.Hostname()
	apiPost("/api/config/push", map[string]any{
		"file_key": fileKey, "content": string(content), "device": hostname,
	})
	fmt.Printf("Pushed %s (%d bytes)\n", fileKey, len(content))
}

func doPull(fileKey string) {
	data := apiPost("/api/config/pull", map[string]any{"file_key": fileKey})
	if content, ok := data["content"].(string); ok {
		fmt.Println(content)
	}
}

func apiGet(path string) map[string]any {
	req, _ := http.NewRequest("GET", serverURL+path, nil)
	if apiKey != "" { req.Header.Set("Authorization", "Bearer "+apiKey) }
	resp, err := http.DefaultClient.Do(req)
	if err != nil { fmt.Printf("Error: %v\n", err); return nil }
	defer resp.Body.Close()
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	return data
}

func apiPost(path string, body map[string]any) map[string]any {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", serverURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" { req.Header.Set("Authorization", "Bearer "+apiKey) }
	resp, err := http.DefaultClient.Do(req)
	if err != nil { fmt.Printf("Error: %v\n", err); return nil }
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error %d: %s\n", resp.StatusCode, string(b))
		return nil
	}
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	return data
}

func printHelp() {
	fmt.Println(`recalla — AI memory CLI

Usage:
  recalla search <query>     Search memories
  recalla s <query>          Search (shortcut)
  recalla save <content>     Save a memory
  recalla context [project]  Restore project context
  recalla c [project]        Context (shortcut)
  recalla projects           List all projects
  recalla briefing [project] Generate daily briefing
  recalla status             Show stats
  recalla push <key> <file>  Push config to server
  recalla pull <key>         Pull config from server
  recalla help               Show this help

Environment:
  RECALLA_URL   Server URL (default: https://peter.recalla.dev)
  RECALLA_KEY   API Key for authentication`)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}
