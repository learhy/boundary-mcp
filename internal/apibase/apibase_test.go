package apibase

import (
	"encoding/json"
	"testing"
)

func TestParseListParams(t *testing.T) {
	args := json.RawMessage(`{"scope_id":"o_123","filter":"name == \"test\"","page_size":50}`)
	p, err := ParseListParams(args)
	if err != nil {
		t.Fatal(err)
	}
	if p.ScopeID != "o_123" {
		t.Errorf("expected scope_id o_123, got %s", p.ScopeID)
	}
	if p.Filter != `name == "test"` {
		t.Errorf("expected filter, got %s", p.Filter)
	}
	if p.PageSize != 50 {
		t.Errorf("expected page_size 50, got %d", p.PageSize)
	}
}

func TestParseListParamsEmpty(t *testing.T) {
	p, err := ParseListParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.ScopeID != "" {
		t.Error("expected empty scope_id")
	}
}

func TestBuildListQuery(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		params   *ListParams
		expect   string
	}{
		{
			name:   "no params",
			base:   "/v1/targets",
			params: &ListParams{},
			expect: "/v1/targets",
		},
		{
			name: "with filter",
			base: "/v1/targets",
			params: &ListParams{
				Filter: `"type" == "ssh"`,
			},
			expect: `/v1/targets?filter=%22type%22+%3D%3D+%22ssh%22`,
		},
		{
			name: "with all params",
			base: "/v1/targets",
			params: &ListParams{
				ScopeID:   "o_123",
				Recursive: true,
				PageSize:  100,
				Filter:    `name == "test"`,
			},
			expect: `/v1/targets?filter=name+%3D%3D+%22test%22&page_size=100&recursive=true&scope_id=o_123`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildListQuery(tt.base, tt.params)
			if got != tt.expect {
				t.Errorf("got: %s\nwant: %s", got, tt.expect)
			}
		})
	}
}

func TestHandleReadResponse(t *testing.T) {
	body := json.RawMessage(`{"id":"ttcp_123","name":"test-target"}`)
	result, err := HandleReadResponse(body, 200, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Error("should not be error")
	}
}

func TestHandleReadResponseError(t *testing.T) {
	body := json.RawMessage(`{"details":[{"messages":["forbidden"]}]}`)
	_, err := HandleReadResponse(body, 403, nil)
	if err == nil {
		t.Error("expected error for 403")
	}
}

func TestTranslateError401(t *testing.T) {
	msg := translateError(401, nil)
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestTranslateError404(t *testing.T) {
	msg := translateError(404, nil)
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}