package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseHosts_JSONMap(t *testing.T) {
	got, err := parseHosts(`{"a.example.com":"a","b.example.com":"b"}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := map[string]string{"a.example.com": "a", "b.example.com": "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseHosts_CSV(t *testing.T) {
	got, err := parseHosts("a.example.com, b.example.com ,c.example.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := map[string]string{
		"a.example.com": "a.example.com",
		"b.example.com": "b.example.com",
		"c.example.com": "c.example.com",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseHosts_RejectsBadJSON(t *testing.T) {
	if _, err := parseHosts("{not json"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFirstURL(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no url", []string{"-X", "POST", "-d", "{}"}, ""},
		{"positional https", []string{"-fsSL", "https://hermes-api.alpheya.com/foo"}, "https://hermes-api.alpheya.com/foo"},
		{"positional http", []string{"http://localhost:8080/x"}, "http://localhost:8080/x"},
		{"after flag", []string{"--header", "X-Foo: bar", "https://api.example.com/v1"}, "https://api.example.com/v1"},
		{"first wins", []string{"https://first.example.com", "https://second.example.com"}, "https://first.example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstURL(tc.args); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMaybeInjectAuth_NoHostsConfig_Passthrough(t *testing.T) {
	t.Setenv(envHosts, "")
	got, err := maybeInjectAuth([]string{"https://example.com"}, neverCalledMint(t))
	if err != nil || got != nil {
		t.Errorf("expected passthrough; got args=%v err=%v", got, err)
	}
}

func TestMaybeInjectAuth_HostNotMatched_Passthrough(t *testing.T) {
	t.Setenv(envHosts, "hermes-api.alpheya.com")
	got, err := maybeInjectAuth([]string{"https://github.com/foo"}, neverCalledMint(t))
	if err != nil || got != nil {
		t.Errorf("expected passthrough; got args=%v err=%v", got, err)
	}
}

func TestMaybeInjectAuth_NoURL_Passthrough(t *testing.T) {
	t.Setenv(envHosts, "hermes-api.alpheya.com")
	got, err := maybeInjectAuth([]string{"--help"}, neverCalledMint(t))
	if err != nil || got != nil {
		t.Errorf("expected passthrough; got args=%v err=%v", got, err)
	}
}

func TestMaybeInjectAuth_MatchedHost_PrependsAuthHeader(t *testing.T) {
	t.Setenv(envHosts, `{"hermes-api.alpheya.com":"hermes"}`)
	t.Setenv(envProfile, "")
	mint := func(service string) (string, error) {
		if service != "hermes" {
			t.Errorf("service = %q, want hermes", service)
		}
		return "fake.jwt.token", nil
	}
	in := []string{"-fsSL", "https://hermes-api.alpheya.com/v1/foo"}
	got, err := maybeInjectAuth(in, mint)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []string{"-H", "Authorization: Bearer fake.jwt.token", "-fsSL", "https://hermes-api.alpheya.com/v1/foo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMaybeInjectAuth_ProfileEnvAppendsToService(t *testing.T) {
	t.Setenv(envHosts, `{"hermes-api.alpheya.com":"hermes"}`)
	t.Setenv(envProfile, "admin")
	mint := func(service string) (string, error) {
		if service != "hermes:admin" {
			t.Errorf("service = %q, want hermes:admin", service)
		}
		return "t", nil
	}
	if _, err := maybeInjectAuth([]string{"https://hermes-api.alpheya.com"}, mint); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestMaybeInjectAuth_MintErrorAborts(t *testing.T) {
	t.Setenv(envHosts, `{"hermes-api.alpheya.com":"hermes"}`)
	t.Setenv(envProfile, "")
	mint := func(service string) (string, error) {
		return "", errors.New("boom")
	}
	if _, err := maybeInjectAuth([]string{"https://hermes-api.alpheya.com"}, mint); err == nil {
		t.Fatal("expected error from mint")
	}
}

func TestMaybeInjectAuth_MalformedHostsAborts(t *testing.T) {
	t.Setenv(envHosts, "{bogus")
	if _, err := maybeInjectAuth([]string{"https://x"}, neverCalledMint(t)); err == nil {
		t.Fatal("expected parse error")
	}
}

func neverCalledMint(t *testing.T) mintFunc {
	return func(service string) (string, error) {
		t.Helper()
		t.Errorf("mint unexpectedly called with %q", service)
		return "", nil
	}
}
