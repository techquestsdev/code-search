// Package regexutil provides utilities for safe regex handling.
package regexutil

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"time"
)

// Limits for regex patterns to prevent ReDoS attacks.
const (
	// MaxPatternLength is the maximum allowed regex pattern length.
	MaxPatternLength = 1000

	// MaxRepetitionCount is the maximum allowed repetition count (e.g., a{1000}).
	MaxRepetitionCount = 100

	// MaxNestedQuantifiers is the maximum depth of nested quantifiers.
	MaxNestedQuantifiers = 3

	// CompileTimeout is the maximum time to spend compiling a regex.
	CompileTimeout = 100 * time.Millisecond
)

// ValidationError represents a regex validation error.
type ValidationError struct {
	Pattern string
	Reason  string
}

func (e *ValidationError) Error() string {
	return "invalid regex pattern: " + e.Reason
}

// ValidatePattern checks if a regex pattern is safe to compile and execute.
// Returns nil if the pattern is safe, or a ValidationError explaining the issue.
func ValidatePattern(pattern string) error {
	// Check length
	if len(pattern) > MaxPatternLength {
		return &ValidationError{
			Pattern: pattern,
			Reason: fmt.Sprintf(
				"pattern too long (%d chars, max %d)",
				len(pattern),
				MaxPatternLength,
			),
		}
	}

	// Check for empty pattern
	if strings.TrimSpace(pattern) == "" {
		return &ValidationError{
			Pattern: pattern,
			Reason:  "pattern cannot be empty",
		}
	}

	// Parse the regex to analyze its structure
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return &ValidationError{
			Pattern: pattern,
			Reason:  fmt.Sprintf("invalid regex syntax: %v", err),
		}
	}

	// Check for dangerous patterns
	if err := checkDangerousPatterns(re, 0); err != nil {
		return err
	}

	return nil
}

// checkDangerousPatterns recursively checks for ReDoS-vulnerable patterns.
func checkDangerousPatterns(re *syntax.Regexp, depth int) error {
	// Check repetition bounds
	if re.Max > MaxRepetitionCount && re.Max != -1 {
		return &ValidationError{
			Reason: fmt.Sprintf(
				"repetition count too high (%d, max %d)",
				re.Max,
				MaxRepetitionCount,
			),
		}
	}

	// Check for nested quantifiers (e.g., (a+)+ or (a*)*)
	// These are the primary cause of ReDoS
	if isQuantifier(re.Op) {
		for _, sub := range re.Sub {
			if containsQuantifier(sub) {
				depth++
				if depth > MaxNestedQuantifiers {
					return &ValidationError{
						Reason: fmt.Sprintf(
							"too many nested quantifiers (max %d)",
							MaxNestedQuantifiers,
						),
					}
				}
			}
		}
	}

	// Recursively check sub-expressions
	for _, sub := range re.Sub {
		if err := checkDangerousPatterns(sub, depth); err != nil {
			return err
		}
	}

	return nil
}

// isQuantifier returns true if the operation is a quantifier.
func isQuantifier(op syntax.Op) bool {
	switch op {
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		return true
	default:
		return false
	}
}

// containsQuantifier returns true if the expression contains a quantifier.
func containsQuantifier(re *syntax.Regexp) bool {
	if isQuantifier(re.Op) {
		return true
	}

	return slices.ContainsFunc(re.Sub, containsQuantifier)
}

// SafeCompile validates and compiles a regex pattern with safety checks.
// Returns the compiled regex or an error if the pattern is unsafe or invalid.
func SafeCompile(pattern string) (*regexp.Regexp, error) {
	// Validate first
	if err := ValidatePattern(pattern); err != nil {
		return nil, err
	}

	// Compile the regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, &ValidationError{
			Pattern: pattern,
			Reason:  fmt.Sprintf("compile error: %v", err),
		}
	}

	return re, nil
}

// SafeCompileWithFlags validates and compiles a regex with the given flags prefix.
// Common flags: "(?i)" for case-insensitive, "(?m)" for multiline.
func SafeCompileWithFlags(pattern string, flags string) (*regexp.Regexp, error) {
	return SafeCompile(flags + pattern)
}

// MustSafeCompile is like SafeCompile but panics on error.
// Only use for patterns known at compile time.
func MustSafeCompile(pattern string) *regexp.Regexp {
	re, err := SafeCompile(pattern)
	if err != nil {
		panic(err)
	}

	return re
}
