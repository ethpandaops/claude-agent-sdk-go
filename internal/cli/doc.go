// Package cli provides CLI discovery, version validation, and command building
// for the Claude Code CLI binary.
//
// This package provides three main capabilities:
//
// # CLI Discovery
//
// The Discoverer interface locates and validates the Claude CLI binary:
//
//	discoverer := cli.NewDiscoverer(&cli.Config{
//	    CliPath: "",           // Optional explicit path
//	    Logger:  slog.Default(),
//	})
//	cliPath, err := discoverer.Discover(ctx)
//
// Discovery searches in the following order:
//  1. Explicit path in Config.CliPath (if provided)
//  2. System PATH
//  3. Common installation directories (/usr/local/bin, /usr/bin, ~/.local/bin)
//
// # Version Validation
//
// During discovery, the CLI version is validated against MinimumVersion (2.0.0).
// A warning is logged if the version is below minimum. Version checking can be
// skipped via Config.SkipVersionCheck or the CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK
// environment variable.
//
// # Command Building
//
// The package provides functions to build CLI command arguments and environment:
//
//	args := cli.BuildArgs("prompt", options, isStreaming)
//	env := cli.BuildEnvironment(options)
package cli
