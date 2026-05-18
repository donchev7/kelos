// kelos-jwt — explicit CLI for minting signed JWTs.
//
// The primary integration point for outbound Alpheya service calls is
// kelos-curl, which transparently injects Authorization on matched
// hosts. This binary exists for the few cases where transparent
// injection is the wrong shape: shell scripts that need to embed a JWT
// in something other than a curl Authorization header, debug commands
// that want to inspect the minted token, and tests.
//
// Reads ALPHEYA_TOKEN_SIGNING_* env vars (see internal/jwt/config.go).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/kelos-dev/kelos/internal/jwt"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "kelos-jwt:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage(errors.New("no command specified"))
	}
	switch args[0] {
	case "sign":
		return runSign(args[1:])
	case "-h", "--help", "help":
		return usage(nil)
	default:
		return usage(fmt.Errorf("unknown subcommand %q", args[0]))
	}
}

func runSign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: kelos-jwt sign <service[:profile]>")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Reads ALPHEYA_TOKEN_SIGNING_* env vars and writes a signed JWT to stdout.")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("expected exactly one service argument")
	}

	cfg, err := jwt.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	signer, err := jwt.NewSigner(cfg)
	if err != nil {
		return err
	}
	token, err := signer.Sign(fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}

func usage(reason error) error {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  kelos-jwt sign <service[:profile]>")
	return reason
}
