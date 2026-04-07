package permissions

import (
	"regexp"
	"testing"
)

func TestRuleMatching(t *testing.T) {
	ruleSet := &RuleSet{
		rules: []Rule{
			{ToolName: "Bash", Pattern: regexp.MustCompile("rm .*"), Action: RuleActionDeny},
			{ToolName: "Read", Pattern: regexp.MustCompile(".*\\.ts"), Action: RuleActionAllow},
		},
	}

	// Should deny rm -rf
	if ruleSet.Check("Bash", "rm -rf /") != RuleActionDeny {
		t.Error("should deny rm -rf")
	}

	// Should allow reading .ts files
	if ruleSet.Check("Read", "main.ts") != RuleActionAllow {
		t.Error("should allow reading .ts files")
	}

	// Should have default behavior for unmatched (RuleActionAsk)
	if ruleSet.Check("Bash", "ls -la") != RuleActionAsk {
		t.Error("should have default behavior for ls")
	}
}

func TestRulePatterns(t *testing.T) {
	tests := []struct {
		pattern     string
		input       string
		shouldMatch bool
	}{
		{".*\\.ts", "main.ts", true},
		{".*\\.ts", "main.js", false},
		{"test.*", "test.go", true},
		{"test.*", "testing.go", true},
	}

	for _, tt := range tests {
		rule := Rule{ToolName: "Test", Pattern: regexp.MustCompile(tt.pattern), Action: RuleActionAllow}
		result := rule.Matches("Test", tt.input)
		if result != tt.shouldMatch {
			t.Errorf("pattern %s with input %s: expected %v, got %v",
				tt.pattern, tt.input, tt.shouldMatch, result)
		}
	}
}

func TestPermissionModes(t *testing.T) {
	// Test different permission modes
	tests := []struct {
		mode      Mode
		shouldAsk bool
	}{
		{ModeDefault, true},
		{ModeDefault, true},
		{ModeBypassPermissions, false},
		{ModeDontAsk, false},
		{ModePlan, true},
	}

	for _, tt := range tests {
		if tt.mode.ShouldAsk() != tt.shouldAsk {
			t.Errorf("mode %s: expected ShouldAsk=%v, got %v",
				tt.mode, tt.shouldAsk, tt.mode.ShouldAsk())
		}
	}
}
