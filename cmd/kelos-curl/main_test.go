package main

import (
	"errors"
	"reflect"
	"testing"
)

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

func TestHasExplicitAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"empty", []string{}, false},
		{"just url", []string{"https://hermes-api.alpheya.com"}, false},
		{"unrelated header", []string{"-H", "X-Foo: bar", "https://x"}, false},
		{"user-pass basic", []string{"-u", "alice:secret", "https://x"}, true},
		{"long user-pass", []string{"--user", "alice:secret", "https://x"}, true},
		{"explicit bearer separate args", []string{"-H", "Authorization: Bearer abc", "https://x"}, true},
		{"explicit bearer long form", []string{"--header", "Authorization: Bearer abc", "https://x"}, true},
		{"explicit bearer packed", []string{"-HAuthorization: Bearer abc", "https://x"}, true},
		{"explicit bearer equals form", []string{"--header=Authorization: Bearer abc", "https://x"}, true},
		{"case-insensitive header", []string{"-H", "authorization: Bearer abc", "https://x"}, true},
		{"leading whitespace tolerated", []string{"-H", " Authorization: Bearer abc", "https://x"}, true},
		{"other header containing word", []string{"-H", "X-Authorization-Method: none", "https://x"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasExplicitAuth(tc.args); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMaybeInjectAuth_NoSigningConfig_Passthrough(t *testing.T) {
	t.Setenv(envIssuer, "")
	got, err := maybeInjectAuth([]string{"https://example.com"}, neverCalledMint(t))
	if err != nil || got != nil {
		t.Errorf("expected passthrough; got args=%v err=%v", got, err)
	}
}

func TestMaybeInjectAuth_NoURL_Passthrough(t *testing.T) {
	t.Setenv(envIssuer, "https://auth.qwlth.dev")
	got, err := maybeInjectAuth([]string{"--help"}, neverCalledMint(t))
	if err != nil || got != nil {
		t.Errorf("expected passthrough; got args=%v err=%v", got, err)
	}
}

func TestMaybeInjectAuth_ExplicitBearerPassthrough(t *testing.T) {
	t.Setenv(envIssuer, "https://auth.qwlth.dev")
	got, err := maybeInjectAuth(
		[]string{"-H", "Authorization: Bearer mine", "https://hermes-api.alpheya.com"},
		neverCalledMint(t),
	)
	if err != nil || got != nil {
		t.Errorf("expected passthrough on explicit auth; got args=%v err=%v", got, err)
	}
}

func TestMaybeInjectAuth_ExplicitBasicPassthrough(t *testing.T) {
	t.Setenv(envIssuer, "https://auth.qwlth.dev")
	got, err := maybeInjectAuth(
		[]string{"-u", "alice:secret", "https://hermes-api.alpheya.com"},
		neverCalledMint(t),
	)
	if err != nil || got != nil {
		t.Errorf("expected passthrough on explicit -u; got args=%v err=%v", got, err)
	}
}

func TestMaybeInjectAuth_SignsEveryHost(t *testing.T) {
	t.Setenv(envIssuer, "https://auth.qwlth.dev")
	t.Setenv(envProfile, "")
	captured := []string{}
	mint := func(service string) (string, error) {
		captured = append(captured, service)
		return "tok-" + service, nil
	}
	cases := []struct{ url, wantService string }{
		{"https://hermes-api.alpheya.com/v1/foo", "hermes-api.alpheya.com"},
		{"https://github.com/foo/bar", "github.com"},
		{"http://localhost:8080/x", "localhost:8080"},
	}
	for _, tc := range cases {
		got, err := maybeInjectAuth([]string{"-fsSL", tc.url}, mint)
		if err != nil {
			t.Fatalf("%s: err: %v", tc.url, err)
		}
		want := []string{"-H", "Authorization: Bearer tok-" + tc.wantService, "-fsSL", tc.url}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got: %v\nwant: %v", tc.url, got, want)
		}
	}
	wantCaptured := []string{"hermes-api.alpheya.com", "github.com", "localhost:8080"}
	if !reflect.DeepEqual(captured, wantCaptured) {
		t.Errorf("services captured by mint = %v, want %v", captured, wantCaptured)
	}
}

func TestMaybeInjectAuth_ProfileEnvAppendsToService(t *testing.T) {
	t.Setenv(envIssuer, "https://auth.qwlth.dev")
	t.Setenv(envProfile, "admin")
	mint := func(service string) (string, error) {
		if service != "hermes-api.alpheya.com:admin" {
			t.Errorf("service = %q, want hermes-api.alpheya.com:admin", service)
		}
		return "t", nil
	}
	if _, err := maybeInjectAuth([]string{"https://hermes-api.alpheya.com"}, mint); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestMaybeInjectAuth_MintErrorAborts(t *testing.T) {
	t.Setenv(envIssuer, "https://auth.qwlth.dev")
	t.Setenv(envProfile, "")
	mint := func(service string) (string, error) {
		return "", errors.New("boom")
	}
	if _, err := maybeInjectAuth([]string{"https://hermes-api.alpheya.com"}, mint); err == nil {
		t.Fatal("expected error from mint")
	}
}

func neverCalledMint(t *testing.T) mintFunc {
	return func(service string) (string, error) {
		t.Helper()
		t.Errorf("mint unexpectedly called with %q", service)
		return "", nil
	}
}
