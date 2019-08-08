package dsi

import (
	"context"
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
