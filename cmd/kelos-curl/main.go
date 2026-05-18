// kelos-curl — curl wrapper that injects an Authorization header
// signed by internal/jwt for every outbound URL.
//
// Same shape as codex/scripts/gh: installed ahead of the real binary on
// PATH (as /usr/local/bin/curl), so every curl invocation from the
// agent picks up auth without the agent having to remember to call a
// helper. The transparent design is the point — pushing activation
// onto the LLM is the failure mode we hit with the initial bash port.
//
// Behavior:
//   - ALPHEYA_TOKEN_SIGNING_ISSUER unset → exec real curl unchanged
//     (signing not configured; e.g., local dev or non-Alpheya agent images).
//   - argv has no http(s) URL → exec real curl unchanged.
//   - argv already carries Authorization (via -H/--header) or Basic
//     auth (-u/--user) → exec real curl unchanged. Agent-explicit auth
//     wins over the wrapper.
//   - Otherwise → LoadConfigFromEnv + mint JWT, prepend
//     `-H Authorization: Bearer <jwt>`, exec real curl.
//   - Signing failure → fail loudly. Never silently fall through to
//     unauthenticated curl — that turns a misconfiguration into an
//     auth-bypass.
package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/kelos-dev/kelos/internal/jwt"
)

const (
	envIssuer   = "ALPHEYA_TOKEN_SIGNING_ISSUER"
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
	// Signing not configured at all → passthrough. Picks one required
	// env var as the gate so a partial configuration still trips
	// LoadConfigFromEnv loudly below instead of silently skipping.
	if os.Getenv(envIssuer) == "" {
		return nil, nil
	}
	if hasExplicitAuth(args) {
		return nil, nil
	}
	target := firstURL(args)
	if target == "" {
		return nil, nil
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return nil, nil
	}
	// Service name fed to the signer is the URL host. The Signer uses
	// this only for `service:profile` lookup against config.Profiles;
	// the host string itself does not land in the JWT.
	service := u.Host
	if profile := os.Getenv(envProfile); profile != "" {
		service = service + ":" + profile
	}
	token, err := mint(service)
	if err != nil {
		return nil, fmt.Errorf("sign jwt for %s: %w", u.Host, err)
	}
	return append([]string{"-H", "Authorization: Bearer " + token}, args...), nil
}

// hasExplicitAuth reports whether argv already carries an Authorization
// header (via `-H`/`--header`) or HTTP Basic credentials (via
// `-u`/`--user`). Either case means the agent is intentionally driving
// auth itself; the wrapper steps aside rather than clobbering it.
func hasExplicitAuth(args []string) bool {
	for i, a := range args {
		switch a {
		case "-u", "--user":
			return true
		case "-H", "--header":
			if i+1 < len(args) && headerIsAuthorization(args[i+1]) {
				return true
			}
		}
		// Short-form `-Hkey:value` packs the header into one token.
		if strings.HasPrefix(a, "-H") && len(a) > 2 && headerIsAuthorization(a[2:]) {
			return true
		}
		if strings.HasPrefix(a, "--header=") && headerIsAuthorization(a[len("--header="):]) {
			return true
		}
	}
	return false
}

func headerIsAuthorization(h string) bool {
	// HTTP header names are case-insensitive. Trim leading whitespace
	// since curl tolerates "Authorization:value" and " Authorization:v".
	h = strings.TrimLeft(h, " \t")
	return len(h) >= len("Authorization:") &&
		strings.EqualFold(h[:len("Authorization:")], "Authorization:")
}

func firstURL(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, "http://") || strings.HasPrefix(a, "https://") {
			return a
		}
	}
	return ""
}
