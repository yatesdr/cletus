package permissions

import (
	"fmt"
	"regexp"
	"strings"
)

// Rule represents a permission rule: ToolName(pattern)
type Rule struct {
	ToolName string
	Pattern  *regexp.Regexp
	Action   RuleAction
}

// RuleAction defines what to do when rule matches
type RuleAction int

const (
	RuleActionAsk RuleAction = iota
	RuleActionAllow
	RuleActionDeny
)

// ParseRule parses a rule string like "Bash(ls *)" or "Read(*.go)"
func ParseRule(ruleStr string) (*Rule, error) {
	ruleStr = strings.TrimSpace(ruleStr)
	if ruleStr == "" {
		return nil, fmt.Errorf("empty rule")
	}

	// Parse "ToolName(pattern)" format
	openParen := strings.Index(ruleStr, "(")
	closeParen := strings.LastIndex(ruleStr, ")")

	if openParen == -1 || closeParen == -1 || closeParen < openParen {
		// No pattern - match all
		return &Rule{
			ToolName: ruleStr,
			Pattern:  nil,
			Action:   RuleActionAsk,
		}, nil
	}

	toolName := ruleStr[:openParen]
	patternStr := ruleStr[openParen+1 : closeParen]

	var pattern *regexp.Regexp
	if patternStr != "*" {
		var err error
		pattern, err = regexp.Compile(patternStr)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}
	}

	return &Rule{
		ToolName: toolName,
		Pattern:  pattern,
		Action:   RuleActionAsk,
	}, nil
}

// Matches checks if this rule applies to the given tool and input
func (r *Rule) Matches(toolName, input string) bool {
	if r.ToolName != toolName {
		return false
	}

	// No pattern means match all
	if r.Pattern == nil {
		return true
	}

	return r.Pattern.MatchString(input)
}

// RuleSet is a collection of rules
type RuleSet struct {
	rules []Rule
}

// NewRuleSet creates a new rule set from strings
func NewRuleSet(ruleStrings []string) (*RuleSet, error) {
	rules := make([]Rule, 0, len(ruleStrings))

	for _, ruleStr := range ruleStrings {
		rule, err := ParseRule(ruleStr)
		if err != nil {
			return nil, fmt.Errorf("parse rule %q: %w", ruleStr, err)
		}
		rules = append(rules, *rule)
	}

	return &RuleSet{rules: rules}, nil
}

// Check determines the action for a tool/input pair
func (rs *RuleSet) Check(toolName, input string) RuleAction {
	// Check in order - first match wins
	for _, rule := range rs.rules {
		if rule.Matches(toolName, input) {
			return rule.Action
		}
	}

	// No match - ask by default
	return RuleActionAsk
}

// AllowRules returns rules that auto-allow
func (rs *RuleSet) AllowRules() []Rule {
	var allow []Rule
	for _, r := range rs.rules {
		if r.Action == RuleActionAllow {
			allow = append(allow, r)
		}
	}
	return allow
}

// DenyRules returns rules that auto-deny
func (rs *RuleSet) DenyRules() []Rule {
	var deny []Rule
	for _, r := range rs.rules {
		if r.Action == RuleActionDeny {
			deny = append(deny, r)
		}
	}
	return deny
}
