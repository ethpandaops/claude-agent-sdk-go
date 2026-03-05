package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/errors"
)

const (
	// MinimumVersion is the minimum required Claude CLI version.
	MinimumVersion = "2.0.0"

	// VersionCheckTimeout is the timeout for the CLI version check command.
	VersionCheckTimeout = 2 * time.Second
)

// Config holds configuration for CLI discovery.
type Config struct {
	// CliPath is an explicit CLI path that skips PATH search.
	// If empty, discovery will search PATH and common locations.
	CliPath string

	// SkipVersionCheck skips version validation during discovery.
	// Can also be controlled via CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK env var.
	SkipVersionCheck bool

	// Logger is an optional logger for discovery operations.
	// If nil, a default no-op logger is used.
	Logger *slog.Logger
}

// Discoverer locates and validates the Claude CLI binary.
type Discoverer interface {
	// Discover locates the Claude CLI binary and validates its version.
	// Returns the absolute path to the CLI binary or an error.
	Discover(ctx context.Context) (string, error)
}

// discoverer implements the Discoverer interface.
type discoverer struct {
	cfg *Config
	log *slog.Logger
}

// Compile-time verification that discoverer implements Discoverer.
var _ Discoverer = (*discoverer)(nil)

// NewDiscoverer creates a new CLI discoverer with the given configuration.
func NewDiscoverer(cfg *Config) Discoverer {
	if cfg == nil {
		cfg = &Config{}
	}

	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	}

	return &discoverer{
		cfg: cfg,
		log: log,
	}
}

// Discover locates the Claude CLI binary and validates its version.
func (d *discoverer) Discover(ctx context.Context) (string, error) {
	d.log.Debug("Discovering Claude CLI binary")

	cliPath, err := d.findCLI()
	if err != nil {
		d.log.Error("Failed to find Claude CLI", "error", err)

		return "", err
	}

	d.log.Debug("Found Claude CLI binary", "cli_path", cliPath)

	// Check version unless skipped
	d.checkVersion(ctx, cliPath)

	return cliPath, nil
}

// findCLI locates the Claude CLI binary.
func (d *discoverer) findCLI() (string, error) {
	// If explicit path provided, use it and only it
	if d.cfg.CliPath != "" {
		d.log.Debug("Using explicit CLI path", "cli_path", d.cfg.CliPath)

		if _, err := os.Stat(d.cfg.CliPath); err == nil {
			return d.cfg.CliPath, nil
		}

		d.log.Debug("Explicit CLI path not found", "cli_path", d.cfg.CliPath)

		return "", &errors.CLINotFoundError{SearchedPaths: []string{d.cfg.CliPath}}
	}

	searchedPaths := make([]string, 0, 4)

	// Search in PATH
	d.log.Debug("Searching for 'claude' in PATH")

	if path, err := exec.LookPath("claude"); err == nil {
		d.log.Debug("Found 'claude' in PATH", "path", path)

		return path, nil
	}

	searchedPaths = append(searchedPaths, "$PATH")

	// Check common locations
	commonPaths := []string{
		"/usr/local/bin/claude",
		"/usr/bin/claude",
	}

	if homeDir, err := os.UserHomeDir(); err == nil {
		commonPaths = append(commonPaths, filepath.Join(homeDir, ".local/bin/claude"))
	}

	for _, path := range commonPaths {
		searchedPaths = append(searchedPaths, path)
		d.log.Debug("Checking common path", "path", path)

		if _, err := os.Stat(path); err == nil {
			d.log.Debug("Found CLI at common path", "path", path)

			return path, nil
		}
	}

	d.log.Warn("Claude CLI not found in any searched paths", "searched_paths", searchedPaths)

	return "", &errors.CLINotFoundError{SearchedPaths: searchedPaths}
}

// checkVersion checks if the Claude CLI version meets minimum requirements.
// Logs a warning if version is below minimum. Errors are silently ignored.
func (d *discoverer) checkVersion(ctx context.Context, cliPath string) {
	// Skip if configured
	if d.cfg.SkipVersionCheck {
		d.log.Debug("Skipping CLI version check (configured)")

		return
	}

	// Skip if env var is set
	if os.Getenv("CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK") != "" {
		d.log.Debug("Skipping CLI version check (CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK set)")

		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, VersionCheckTimeout)
	defer cancel()

	// Run cli -v
	cmd := exec.CommandContext(ctx, cliPath, "-v")

	output, err := cmd.Output()
	if err != nil {
		// Silently ignore errors
		d.log.Debug("CLI version check failed", "error", err)

		return
	}

	// Parse version with regex: extract "X.Y.Z"
	versionStr := strings.TrimSpace(string(output))
	re := regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+)`)

	match := re.FindStringSubmatch(versionStr)
	if match == nil {
		d.log.Debug("Could not parse CLI version", "output", versionStr)

		return
	}

	version := match[1]
	if compareVersions(version, MinimumVersion) < 0 {
		d.log.Warn("Claude Code version is unsupported in the Agent SDK",
			"version", version,
			"minimum_required", MinimumVersion,
		)

		fmt.Fprintf(os.Stderr,
			"Warning: Claude Code version %s is unsupported in the Agent SDK. "+
				"Minimum required version is %s. Some features may not work correctly.\n",
			version, MinimumVersion,
		)
	} else {
		d.log.Debug("CLI version check passed", "version", version, "minimum", MinimumVersion)
	}
}

// compareVersions compares two semantic versions.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := range 3 {
		aNum := 0
		bNum := 0

		if i < len(aParts) {
			aNum, _ = strconv.Atoi(aParts[i])
		}

		if i < len(bParts) {
			bNum, _ = strconv.Atoi(bParts[i])
		}

		if aNum < bNum {
			return -1
		}

		if aNum > bNum {
			return 1
		}
	}

	return 0
}
