package garden

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInteractionRules(t *testing.T) {
	flags := []Flag{{Name: "yes", Type: "bool", Default: "false"}, {Name: "keyring-backend", Type: "string", Default: "os"}, {Name: "generate-only", Type: "bool"}, {Name: "dry-run", Type: "bool"}, {Name: "recover", Type: "bool"}}
	for _, tc := range []struct {
		path, help    string
		values        map[string]FieldValue
		level, source string
	}{
		{"tx gov draft-proposal", "", nil, "recommended", "known-command"},
		{"tx gov draft-proposal", "", map[string]FieldValue{"flag:yes": {true, "true"}}, "recommended", "known-command"},
		{"tx gov submit-proposal", "", nil, "conditional", "flags"},
		{"tx bank send", "", map[string]FieldValue{"flag:yes": {true, "true"}}, "conditional", "flags"},
		{"tx bank send", "", map[string]FieldValue{"flag:yes": {true, "true"}, "flag:keyring-backend": {true, "test"}}, "normal", "flags"},
		{"tx bank send", "", map[string]FieldValue{"flag:yes": {false, "true"}, "flag:keyring-backend": {true, "test"}}, "conditional", "flags"},
		{"tx bank send", "", map[string]FieldValue{"flag:generate-only": {true, "true"}}, "conditional", "flags"},
		{"tx bank send", "", map[string]FieldValue{"flag:generate-only": {true, "true"}, "flag:keyring-backend": {true, "test"}}, "normal", "flags"},
		{"query bank balances", "Usage:\n demo query bank balances\nFlags:\n --yes Skip confirmation prompt", nil, "normal", "known-command"},
		{"keys add", "", map[string]FieldValue{"flag:recover": {true, "true"}}, "recommended", "flags"},
		{"custom", "Reads data from stdin\nUsage:\n demo custom", nil, "conditional", "help"},
	} {
		t.Run(tc.path+tc.level+tc.help, func(t *testing.T) {
			c := &Command{Path: strings.Fields(tc.path), Help: tc.help, Flags: flags}
			got := ResolveInteraction(c, tc.values)
			if got.Level != tc.level || got.Source != tc.source || len(got.Reasons) == 0 {
				t.Fatalf("%+v", got)
			}
		})
	}
	if got := AnalyzeInteraction(&Command{Path: []string{"custom"}}); got.Level != "unknown" {
		t.Fatal(got)
	}
}

func TestInteractionDescriptorRoundTrip(t *testing.T) {
	c := ParseHelp([]string{"tx", "gov", "draft-proposal"}, "Draft proposal\nUsage:\n demo tx gov draft-proposal [flags]\n")
	path := filepath.Join(t.TempDir(), "descriptor.json")
	if err := Save(path, Descriptor{Schema: Schema, Root: c}); err != nil {
		t.Fatal(err)
	}
	var restored Descriptor
	if err := Load(path, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Root.Interaction.Level != "recommended" || restored.Root.Interaction.Source != "known-command" {
		t.Fatal(restored.Root.Interaction)
	}
}
