package permission

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateToDict_Minimal(t *testing.T) {
	update := &Update{
		Type: UpdateTypeSetMode,
	}

	got := update.ToDict()

	require.Equal(t, map[string]any{
		"type": string(UpdateTypeSetMode),
	}, got)
}

func TestUpdateToDict_Full(t *testing.T) {
	ruleContent := "allow all"
	behavior := BehaviorAllow
	mode := ModeAcceptEdits
	destination := UpdateDestProjectSettings

	update := &Update{
		Type: UpdateTypeAddRules,
		Rules: []*RuleValue{
			{
				ToolName:    "Read",
				RuleContent: &ruleContent,
			},
			{
				ToolName: "Write",
			},
		},
		Behavior:    &behavior,
		Mode:        &mode,
		Directories: []string{"/workspace", "/tmp"},
		Destination: &destination,
	}

	got := update.ToDict()

	require.Equal(t, map[string]any{
		"type":        string(UpdateTypeAddRules),
		"destination": string(UpdateDestProjectSettings),
		"rules": []map[string]any{
			{
				"toolName":    "Read",
				"ruleContent": "allow all",
			},
			{
				"toolName": "Write",
			},
		},
		"behavior":    string(BehaviorAllow),
		"mode":        string(ModeAcceptEdits),
		"directories": []string{"/workspace", "/tmp"},
	}, got)
}

func TestResultBehaviors(t *testing.T) {
	allow := &ResultAllow{}
	deny := &ResultDeny{}

	require.Equal(t, "allow", allow.GetBehavior())
	require.Equal(t, "deny", deny.GetBehavior())
}
