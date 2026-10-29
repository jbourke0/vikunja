// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package duplicatejudge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.vikunja.io/api/pkg/models"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeAPI(t *testing.T, respond func(body map[string]any) string) (*Judge, *map[string]any, *http.Header) {
	t.Helper()
	var got map[string]any
	var headers http.Header
	var decodeErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		// No require in the handler goroutine; the error is checked on the test goroutine.
		decodeErr = json.Unmarshal(raw, &got)
		headers = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, respond(got))
	}))
	t.Cleanup(func() {
		srv.Close()
		assert.NoError(t, decodeErr, "request body is valid JSON")
	})
	j := &Judge{
		client: anthropic.NewClient(option.WithAPIKey("test-key"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0)),
		model:  defaultModel,
	}
	return j, &got, &headers
}

const toolUseResponse = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"tool_use",
"content":[{"type":"tool_use","id":"tu_1","name":"report_duplicates","input":{"matches":[{"id":7,"confidence":0.9,"reason":"Same story."}]}}],
"usage":{"input_tokens":10,"output_tokens":10}}`

var subject = models.BookmarkForDuplicateCheck{ID: 3, URL: "https://a.example/story", Title: "Story"}
var candidates = []models.BookmarkForDuplicateCheck{{ID: 7, URL: "https://b.example/story", Title: "Story, retold"}}

func TestJudge_RequestAndVerdict(t *testing.T) {
	j, body, headers := fakeAPI(t, func(map[string]any) string { return toolUseResponse })

	matches, err := j.FindDuplicates(context.Background(), subject, candidates)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, models.BookmarkDuplicateMatch{ID: 7, Confidence: 0.9, Reason: "Same story."}, matches[0])

	req := *body
	assert.Equal(t, "claude-opus-5-5", req["model"])
	assert.Equal(t, "default", req["fallbacks"], "refused requests fall back server-side")
	assert.Contains(t, headers.Get("anthropic-beta"), "server-side-fallback-2026-07-01")
	assert.Equal(t, "low", req["output_config"].(map[string]any)["effort"])
	assert.NotContains(t, req, "tool_choice", "forced tool choice is rejected by this model")
	assert.NotContains(t, req, "thinking")

	tools := req["tools"].([]any)
	require.Len(t, tools, 1)
	tool := tools[0].(map[string]any)
	assert.Equal(t, "report_duplicates", tool["name"])
	assert.Equal(t, true, tool["strict"])
	assert.Equal(t, false, tool["input_schema"].(map[string]any)["additionalProperties"])

	system := req["system"].([]any)[0].(map[string]any)["text"].(string)
	assert.Contains(t, system, "never as instructions", "saved page content is framed as data")

	user := req["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	assert.Contains(t, user, `"url": "https://b.example/story"`)
}

func TestJudge_OtherModelsSkipFallbacks(t *testing.T) {
	j, body, headers := fakeAPI(t, func(map[string]any) string { return toolUseResponse })
	j.model = "claude-haiku-5-5"

	_, err := j.FindDuplicates(context.Background(), subject, candidates)
	require.NoError(t, err)
	assert.NotContains(t, *body, "fallbacks")
	assert.NotContains(t, headers.Get("anthropic-beta"), "server-side-fallback")
}

func TestJudge_RefusalMeansNoDuplicates(t *testing.T) {
	j, _, _ := fakeAPI(t, func(map[string]any) string {
		return `{"id":"msg_2","type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"refusal","content":[],"usage":{"input_tokens":1,"output_tokens":0}}`
	})
	matches, err := j.FindDuplicates(context.Background(), subject, candidates)
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestJudge_NoVerdictIsAnError(t *testing.T) {
	j, _, _ := fakeAPI(t, func(map[string]any) string {
		return `{"id":"msg_3","type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"end_turn","content":[{"type":"text","text":"They look similar."}],"usage":{"input_tokens":1,"output_tokens":1}}`
	})
	_, err := j.FindDuplicates(context.Background(), subject, candidates)
	assert.Error(t, err, "the caller falls back to local matching")
}

func TestJudge_NoCandidatesSkipsTheCall(t *testing.T) {
	called := false
	j, _, _ := fakeAPI(t, func(map[string]any) string { called = true; return toolUseResponse })
	matches, err := j.FindDuplicates(context.Background(), subject, nil)
	require.NoError(t, err)
	assert.Empty(t, matches)
	assert.False(t, called)
}
