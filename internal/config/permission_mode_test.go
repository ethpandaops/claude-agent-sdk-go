package config

import "testing"

func TestNormalizePermissionMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "legacy acceptAll", in: "acceptAll", want: "bypassPermissions"},
		{name: "legacy prompt", in: "prompt", want: "default"},
		{name: "current mode unchanged", in: "acceptEdits", want: "acceptEdits"},
		{name: "empty unchanged", in: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := NormalizePermissionMode(tc.in)
			if got != tc.want {
				t.Fatalf("NormalizePermissionMode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
