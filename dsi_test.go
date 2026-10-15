package dsi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test_Traceme(t *testing.T) {
	patchconfig = []*patch{
		{
			What:   "test",
			Match:  "1",
			Return: true,
		},
		{
			What:  "test2",
			Match: "1",
			Patch: true,
		},
		{
			What:   "test3",
			Match:  "1",
			Patch:  true,
			Repeat: 3,
		},
	}

	var res, called bool

	res, called = false, false
	p := 1
	Traceme(context.Background(), "test", map[string]interface{}{"a": &p}, func() { called = true }, &res)
	if !res {
		t.Errorf("res was not set to true")
	}
	if called {
		t.Errorf("return should not call")
	}

	res, called = false, false
	Traceme(context.Background(), "test2", map[string]interface{}{}, func() { called = true }, &res)
	if !res {
		t.Errorf("res was not set to true")
	}
	if !called {
		t.Errorf("patch should call")
	}

	t.Run("repeats", func(t *testing.T) {
		Traceme(context.Background(), "test3", map[string]interface{}{}, func() { called = true }, &res)
		Traceme(context.Background(), "test3", map[string]interface{}{}, func() { called = true }, &res)

		res, called = false, false
		Traceme(context.Background(), "test3", map[string]interface{}{}, func() { called = true }, &res)
		if !res {
			t.Errorf("res was not set to true")
		}
		if !called {
			t.Errorf("patch should call")
		}

		res, called = false, false
		Traceme(context.Background(), "test3", map[string]interface{}{}, func() { called = true }, &res)
		if res {
			t.Errorf("res should not be patched again")
		}
		if !called {
			t.Errorf("after repetition should call")
		}
	})
}

func Test_Scenarios_CDN(t *testing.T) {
	req := httptest.NewRequest("GET", "/?scenarios=show", nil)
	rec := httptest.NewRecorder()
	scenarios(rec, req)
	body := rec.Body.String()
	if strings.Contains(body, "pagecdn.io") {
		t.Errorf("scenarios response contains unsafe pagecdn.io CDN link")
	}
	if !strings.Contains(body, "https://cdnjs.cloudflare.com/ajax/libs/ace/1.4.5/ace.js") {
		t.Errorf("scenarios response does not contain cloudflare ace.js CDN link")
	}
}
