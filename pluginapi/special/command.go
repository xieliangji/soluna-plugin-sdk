package special

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// ErrNotImplemented means a provider has not implemented a business method.
// It must never be interpreted as a passed test or a completed cleanup.
var ErrNotImplemented = errors.New("special: business method not implemented")

// Main is the standard process entry point for special plugins. It owns command
// parsing, manifest creation, instance binding and stdio protocol startup.
// Call it as os.Exit(special.Main(provider)); diagnostics go to stderr only.
func Main(provider Provider) int {
	err := command(context.Background(), provider, os.Args[1:], os.Stderr)
	if err != nil {
		slog.New(slog.NewTextHandler(os.Stderr, nil)).Error("专项插件退出", "error", err)
		return 1
	}
	return 0
}

func command(ctx context.Context, provider Provider, args []string, diagnostics io.Writer) error {
	flags := flag.NewFlagSet("special-plugin", flag.ContinueOnError)
	flags.SetOutput(diagnostics)
	manifest := flags.String("manifest", "", "write a manifest for the built binary")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("special: unexpected arguments")
	}
	if provider == nil {
		return fmt.Errorf("special: provider required")
	}
	descriptor := provider.Describe()
	if err := descriptor.Validate(); err != nil {
		return err
	}
	if *manifest != "" {
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		return WriteManifest(*manifest, binary, descriptor.ID, descriptor)
	}
	return Serve(ctx, Stdio{Reader: os.Stdin, Writer: os.Stdout}, os.Getenv("SOLUNA_SPECIAL_INSTANCE"), provider)
}
