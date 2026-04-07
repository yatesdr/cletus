package util

import (
	"fmt"
)

// Diff represents a diff between two strings
type Diff struct {
	From string
	To   string
	Type string // equal, insert, delete
}

// ComputeDiff computes the diff between two strings
func ComputeDiff(old, new string) []Diff {
	if old == new {
		return []Diff{{From: old, To: new, Type: "equal"}}
	}
	
	oldLines := splitLines(old)
	newLines := splitLines(new)
	
	return computeLineDiff(oldLines, newLines)
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	result := make([]string, 0)
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func computeLineDiff(oldLines, newLines []string) []Diff {
	lcs := longestCommonSubsequence(oldLines, newLines)
	
	var diffs []Diff
	oldIdx, newIdx, lcsIdx := 0, 0, 0
	
	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		if lcsIdx < len(lcs) {
			for oldIdx < len(oldLines) && oldLines[oldIdx] != lcs[lcsIdx] {
				diffs = append(diffs, Diff{From: oldLines[oldIdx], Type: "delete"})
				oldIdx++
			}
			for newIdx < len(newLines) && newLines[newIdx] != lcs[lcsIdx] {
				diffs = append(diffs, Diff{To: newLines[newIdx], Type: "insert"})
				newIdx++
			}
			if oldIdx < len(oldLines) && newIdx < len(newLines) {
				diffs = append(diffs, Diff{From: oldLines[oldIdx], To: newLines[newIdx], Type: "equal"})
				oldIdx++
				newIdx++
				lcsIdx++
			}
		} else {
			for oldIdx < len(oldLines) {
				diffs = append(diffs, Diff{From: oldLines[oldIdx], Type: "delete"})
				oldIdx++
			}
			for newIdx < len(newLines) {
				diffs = append(diffs, Diff{To: newLines[newIdx], Type: "insert"})
				newIdx++
			}
		}
	}
	return diffs
}

func longestCommonSubsequence(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	
	var lcs []string
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs = append(lcs, a[i-1])
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	for i, j := 0, len(lcs)-1; i < j; i, j = i+1, j-1 {
		lcs[i], lcs[j] = lcs[j], lcs[i]
	}
	return lcs
}

// FormatDiff formats diff for display
func FormatDiff(diffs []Diff) string {
	var result string
	for _, d := range diffs {
		switch d.Type {
		case "equal":
			result += fmt.Sprintf("  %s\n", d.From)
		case "delete":
			result += fmt.Sprintf("- %s\n", d.From)
		case "insert":
			result += fmt.Sprintf("+ %s\n", d.To)
		}
	}
	return result
}

// GetUnifiedDiff returns unified diff format
func GetUnifiedDiff(old, new, oldName, newName string) string {
	diffs := ComputeDiff(old, new)
	var result string
	result += fmt.Sprintf("--- %s\n", oldName)
	result += fmt.Sprintf("+++ %s\n", newName)
	for _, d := range diffs {
		switch d.Type {
		case "equal":
			result += fmt.Sprintf(" %s\n", d.From)
		case "delete":
			result += fmt.Sprintf("-%s\n", d.From)
		case "insert":
			result += fmt.Sprintf("+%s\n", d.To)
		}
	}
	return result
}
