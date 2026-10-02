package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/lib/llm"
)

// This is the Taco regression in the Direct Brain itself. A web/recipe request must be able to go
// through the real OpenAI-compatible tool loop, SearXNG search, page reader and back to the model.
// The model is fake on purpose: this test proves the device executes the requested information path
// rather than accidentally routing it through a device-control implementation.
func TestDirectRecipeRequestRunsSearchAndReadsResult(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	var (
		mu          sync.Mutex
		llmCalls    int
		searchCalls int
		pageCalls   int
		server      *httptest.Server
	)

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			mu.Lock()
			llmCalls++
			call := llmCalls
			mu.Unlock()

			var req struct {
				Messages []llm.Message `json:"messages"`
				Tools    []struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tools"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode LLM request: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")

			switch call {
			case 1:
				if len(req.Messages) < 2 || req.Messages[0].Role != "system" || req.Messages[len(req.Messages)-1].Content != "Suche im Web nach einem Taco-Rezept" {
					t.Errorf("unexpected first LLM messages: %+v", req.Messages)
				}
				system := req.Messages[0].Content
				for _, want := range []string{
					"Never turn an informational question into a device action",
					"find a recipe, use web_search",
				} {
					if !strings.Contains(system, want) {
						t.Errorf("system prompt missing %q", want)
					}
				}
				if !requestOffersTool(req.Tools, "web_search") || !requestOffersTool(req.Tools, "read_page") {
					t.Errorf("web tools were not offered: %+v", req.Tools)
				}
				writeToolCall(t, w, "search-1", "web_search", `{"query":"Taco Rezept"}`)

			case 2:
				tool := messageByToolCallID(req.Messages, "search-1")
				if tool == nil || !strings.Contains(tool.Content, server.URL+"/recipe") {
					t.Errorf("search result was not returned to model: %+v", tool)
				}
				writeToolCall(t, w, "page-1", "read_page", fmt.Sprintf(`{"url":%q}`, server.URL+"/recipe"))

			case 3:
				tool := messageByToolCallID(req.Messages, "page-1")
				if tool == nil || !strings.Contains(tool.Content, "Mais-Tortillas") || !strings.Contains(tool.Content, "Salsa") {
					t.Errorf("page text was not returned to model: %+v", tool)
				}
				writeAssistant(t, w, "Ein Taco wird mit einer Tortilla, einer Füllung und Salsa serviert.")

			default:
				t.Errorf("unexpected LLM call %d", call)
				http.Error(w, "too many calls", http.StatusInternalServerError)
			}

		case "/search":
			mu.Lock()
			searchCalls++
			mu.Unlock()
			if got := r.URL.Query().Get("format"); got != "json" {
				t.Errorf("search format = %q, want json", got)
			}
			if got := r.URL.Query().Get("q"); got != "Taco Rezept" {
				t.Errorf("search query = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{
					"title":   "Ein einfaches Taco-Rezept",
					"url":     server.URL + "/recipe",
					"content": "Tacos mit Bohnen, Gemüse und Salsa.",
				}},
			})

		case "/recipe":
			mu.Lock()
			pageCalls++
			mu.Unlock()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><body><main><h1>Taco-Rezept</h1><p>Mais-Tortillas erwärmen.</p><p>Mit Bohnen und Gemüse füllen und mit Salsa servieren.</p></main></body></html>`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if err := config.Set().Brain().Set(config.Brain{
		Mode:   config.BrainDirect,
		STT:    "127.0.0.1:1",
		TTS:    "127.0.0.1:2",
		LLM:    server.URL,
		Model:  "fake",
		Search: server.URL,
	}); err != nil {
		t.Fatal(err)
	}

	a := &Assistant{}
	got, err := a.Think(context.Background(), "Suche im Web nach einem Taco-Rezept")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Ein Taco wird mit einer Tortilla, einer Füllung und Salsa serviert."; got != want {
		t.Fatalf("reply = %q, want %q", got, want)
	}

	mu.Lock()
	defer mu.Unlock()
	if llmCalls != 3 || searchCalls != 1 || pageCalls != 1 {
		t.Fatalf("calls: llm=%d search=%d page=%d, want 3/1/1", llmCalls, searchCalls, pageCalls)
	}
}

// A general-information answer is also a valid Direct Brain outcome without any tool call. This
// guards the opposite half of the historical Taco failure: an informational noun must not require
// some local device action merely because device tools are available to the model.
func TestDirectInformationQuestionCanAnswerWithoutDeviceAction(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	var llmCalls, searchCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			llmCalls++
			var req struct {
				Messages []llm.Message `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if len(req.Messages) == 0 || req.Messages[len(req.Messages)-1].Content != "Was ist ein Taco?" {
				t.Fatalf("unexpected messages: %+v", req.Messages)
			}
			writeAssistant(t, w, "Ein Taco ist eine gefüllte Tortilla aus der mexikanischen Küche.")
		case "/search":
			searchCalls++
			http.Error(w, "search should not be needed", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if err := config.Set().Brain().Set(config.Brain{
		Mode:   config.BrainDirect,
		STT:    "127.0.0.1:1",
		TTS:    "127.0.0.1:2",
		LLM:    server.URL,
		Model:  "fake",
		Search: server.URL,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := (&Assistant{}).Think(context.Background(), "Was ist ein Taco?")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Ein Taco ist eine gefüllte Tortilla aus der mexikanischen Küche." {
		t.Fatalf("unexpected reply: %q", got)
	}
	if llmCalls != 1 || searchCalls != 0 {
		t.Fatalf("calls: llm=%d search=%d, want 1/0", llmCalls, searchCalls)
	}
}

func requestOffersTool(tools []struct {
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}, name string) bool {
	for _, tool := range tools {
		if tool.Function.Name == name {
			return true
		}
	}
	return false
}

func messageByToolCallID(messages []llm.Message, id string) *llm.Message {
	for i := range messages {
		if messages[i].Role == "tool" && messages[i].ToolCallID == id {
			return &messages[i]
		}
	}
	return nil
}

func writeToolCall(t *testing.T, w http.ResponseWriter, id, name, args string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{{
			"message": map[string]any{
				"role":    "assistant",
				"content": "",
				"tool_calls": []map[string]any{{
					"id":   id,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": args,
					},
				}},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
}

func writeAssistant(t *testing.T, w http.ResponseWriter, content string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{{
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
}
