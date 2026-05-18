// kelos-curl — transparent curl wrapper that injects an Authorization
// header for hosts listed in ALPHEYA_TOKEN_SIGNING_HOSTS.
//
// This is the same shape as codex/scripts/gh: install it ahead of the
// real binary on PATH (as /usr/local/bin/curl), and every curl
// invocation from the agent picks up auth without the agent having to
// remember to call a helper. The transparent design is the point —
// pushing activation onto the LLM is the failure mode we hit with the
// initial bash sign-jwt port.
//
// Behavior:
//   - ALPHEYA_TOKEN_SIGNING_HOSTS unset → exec real curl unchanged.
//   - argv contains no http(s) URL → exec real curl unchanged.
//   - URL host not in the HOSTS map → exec real curl unchanged.
//   - URL host in HOSTS → mint JWT via internal/jwt, prepend
//     `-H Authorization: Bearer <jwt>`, exec real curl.
//   - HOSTS set but malformed, OR signing fails for a matched host →
//     fail loudly. We never silently fall through to unauthenticated
//     curl on a matched host, because that turns a misconfiguration
//     into an auth-bypass.
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/kelos-dev/kelos/internal/jwt"
)

const (
	envHosts    = "ALPHEYA_TOKEN_SIGNING_HOSTS"
	envProfile  = "ALPHEYA_TOKEN_PROFILE"
	envCurlBin  = "KELOS_CURL_REAL"
	defaultCurl = "/usr/bin/curl"
)

func main() {
	args := os.Args[1:]

	augmented, err := maybeInjectAuth(args, mintTokenFromEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kelos-curl:", err)
		os.Exit(2)
	}
	if augmented != nil {
		args = augmented
	}

	realCurl := os.Getenv(envCurlBin)
	if realCurl == "" {
		realCurl = defaultCurl
	}
	if err := syscall.Exec(realCurl, append([]string{"curl"}, args...), os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "kelos-curl: exec", realCurl+":", err)
		os.Exit(127)
	}
}

// mintFunc is a tiny seam so tests can drive maybeInjectAuth without
// touching env vars or filesystem.
type mintFunc func(service string) (string, error)

func mintTokenFromEnv(service string) (string, error) {
	cfg, err := jwt.LoadConfigFromEnv()
	if err != nil {
		return "", err
	}
	signer, err := jwt.NewSigner(cfg)
	if err != nil {
		return "", err
	}
	return signer.Sign(service)
}

// maybeInjectAuth returns nil to mean "passthrough — no rewrite". A
// non-nil slice is the new argv to exec with. Errors abort exec.
func maybeInjectAuth(args []string, mint mintFunc) ([]string, error) {
	hostsRaw := os.Getenv(envHosts)
	if hostsRaw == "" {
		return nil, nil
	}
	hosts, err := parseHosts(hostsRaw)
	if err != nil {
		return nil, err
	}
	target := firstURL(args)
	if target == "" {
		return nil, nil
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return nil, nil
	}
	service, ok := hosts[u.Host]
	if !ok {
		return nil, nil
	}
	if profile := os.Getenv(envProfile); profile != "" {
		service = service + ":" + profile
	}
	token, err := mint(service)
	if err != nil {
		return nil, fmt.Errorf("sign jwt for %s: %w", u.Host, err)
	}
	return append([]string{"-H", "Authorization: Bearer " + token}, args...), nil
}

// parseHosts accepts two formats:
//   - JSON object: {"hermes-api.alpheya.com":"hermes",...}  → service name decoupled from host
//   - CSV string:  hermes-api.alpheya.com,facade.alpheya.com → service name = host
func parseHosts(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{") {
		var m map[string]string
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil, fmt.Errorf("%s: %w", envHosts, err)
		}
		return m, nil
	}
	m := map[string]string{}
	for _, h := range strings.Split(raw, ",") {
		h = strings.TrimSpace(h)
		if h != "" {
			m[h] = h
		}
	}
	return m, nil
}

// firstURL scans argv for the first http/https token. Curl accepts URLs
// positionally and via --url; both surface as a token starting with
// http:// or https://, so a substring scan is enough — we don't need a
// full curl-flag parser to identify the target host.
func firstURL(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, "http://") || strings.HasPrefix(a, "https://") {
			return a
		}
	}
	return ""
}
