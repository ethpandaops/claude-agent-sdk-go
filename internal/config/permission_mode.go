package config

// NormalizePermissionMode maps legacy permission mode names to current CLI values.
//
// Legacy mappings:
//   - "acceptAll" -> "bypassPermissions"
//   - "prompt" -> "default"
func NormalizePermissionMode(mode string) string {
	switch mode {
	case "acceptAll":
		return "bypassPermissions"
	case "prompt":
		return "default"
	default:
		return mode
	}
}
