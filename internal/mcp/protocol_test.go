package mcp

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestTextResult(t *testing.T) {
	r := TextResult("hello")
	if r.Content[0].Text != "hello" {
		t.Errorf("expected 'hello', got %s", r.Content[0].Text)
	}
	if r.IsError {
		t.Error("should not be error")
	}
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult("bad thing")
	if r.Content[0].Text != "bad thing" {
		t.Errorf("expected 'bad thing', got %s", r.Content[0].Text)
	}
	if !r.IsError {
		t.Error("should be error")
	}
}

func TestNewResponse(t *testing.T) {
	id := json.RawMessage(`1`)
	resp, err := NewResponse(id, map[string]string{"status": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.ID) != `1` {
		t.Errorf("expected id '1', got %s", resp.ID)
	}
	if resp.Error != nil {
		t.Error("should not have error")
	}
	var result map[string]string
	json.Unmarshal(resp.Result, &result)
	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %s", result["status"])
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(json.RawMessage(`1`), ErrorCodeMethodNotFound, "no such method")
	if resp.Error.Code != ErrorCodeMethodNotFound {
		t.Errorf("expected code %d, got %d", ErrorCodeMethodNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "no such method" {
		t.Errorf("expected 'no such method', got %s", resp.Error.Message)
	}
}

func TestWriteMessage(t *testing.T) {
	var buf bytes.Buffer
	resp, _ := NewResponse(json.RawMessage(`1`), map[string]string{"ok": "true"})
	if err := WriteMessage(&buf, resp); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		t.Error("message should end with newline")
	}
	var msg Response
	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.JSONRPC != "2.0" {
		t.Errorf("expected JSONRPC 2.0, got %s", msg.JSONRPC)
	}
}