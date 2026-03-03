package config

import (
	"os"
	"testing"
)

func TestLoad_Idempotent(t *testing.T) {
	Load()
	Load()
	Load()
	if !cfg.loaded {
		t.Fatal("cfg.loaded should be true after Load()")
	}
}

func TestGetBool_TrueValues(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "True", "yes", "YES", "Yes"} {
		os.Setenv("TEST_BOOL", v)
		if !getBool("TEST_BOOL", "false") {
			t.Errorf("getBool with %q should be true", v)
		}
	}
	os.Unsetenv("TEST_BOOL")
}

func TestGetBool_FalseValues(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "random", ""} {
		os.Setenv("TEST_BOOL", v)
		if getBool("TEST_BOOL", "false") {
			t.Errorf("getBool with %q should be false", v)
		}
	}
	os.Unsetenv("TEST_BOOL")
}

func TestGetBool_DefaultWhenEmpty(t *testing.T) {
	os.Unsetenv("TEST_BOOL_EMPTY")
	if !getBool("TEST_BOOL_EMPTY", "true") {
		t.Error("getBool should use default \"true\" when env is unset")
	}
	if getBool("TEST_BOOL_EMPTY", "false") {
		t.Error("getBool should use default \"false\" when env is unset")
	}
}

func TestGet_TrimsWhitespace(t *testing.T) {
	os.Setenv("TEST_TRIM", "  value  ")
	got := get("TEST_TRIM")
	if got != "value" {
		t.Errorf("get should trim whitespace: got %q", got)
	}
	os.Unsetenv("TEST_TRIM")
}

func TestGet_EmptyWhenUnset(t *testing.T) {
	os.Unsetenv("TEST_UNSET_VAR")
	got := get("TEST_UNSET_VAR")
	if got != "" {
		t.Errorf("get on unset var should return empty, got %q", got)
	}
}

func TestExportedGet(t *testing.T) {
	os.Setenv("TEST_EXPORTED", "hello")
	got := Get("TEST_EXPORTED")
	if got != "hello" {
		t.Errorf("Get should return env value, got %q", got)
	}
	os.Unsetenv("TEST_EXPORTED")
}

func TestExportedGetBool(t *testing.T) {
	os.Setenv("TEST_EXPORTED_BOOL", "1")
	if !GetBool("TEST_EXPORTED_BOOL", "false") {
		t.Error("GetBool should return true for \"1\"")
	}
	os.Unsetenv("TEST_EXPORTED_BOOL")
}
