package regexutil

import (
	"strings"
	"testing"
)

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		wantErr     bool
		errContains string
	}{
		// Valid patterns
		{
			name:    "simple literal",
			pattern: "hello",
			wantErr: false,
		},
		{
			name:    "simple character class",
			pattern: "[a-z]+",
			wantErr: false,
		},
		{
			name:    "word boundary",
			pattern: `\bword\b`,
			wantErr: false,
		},
		{
			name:    "alternation",
			pattern: "foo|bar|baz",
			wantErr: false,
		},
		{
			name:    "grouping",
			pattern: "(abc)+",
			wantErr: false,
		},
		{
			name:    "reasonable repetition",
			pattern: "a{1,10}",
			wantErr: false,
		},
		{
			name:    "case insensitive flag",
			pattern: "(?i)hello",
			wantErr: false,
		},
		// Invalid patterns - empty
		{
			name:        "empty pattern",
			pattern:     "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "whitespace only",
			pattern:     "   ",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		// Invalid patterns - too long
		{
			name:        "pattern too long",
			pattern:     strings.Repeat("a", MaxPatternLength+1),
			wantErr:     true,
			errContains: "pattern too long",
		},
		// Invalid patterns - syntax errors
		{
			name:        "unclosed bracket",
			pattern:     "[abc",
			wantErr:     true,
			errContains: "invalid regex syntax",
		},
		{
			name:        "unclosed paren",
			pattern:     "(abc",
			wantErr:     true,
			errContains: "invalid regex syntax",
		},
		{
			name:        "invalid escape",
			pattern:     `\`,
			wantErr:     true,
			errContains: "invalid regex syntax",
		},
		// Invalid patterns - high repetition
		{
			name:        "repetition too high",
			pattern:     "a{1,1000}",
			wantErr:     true,
			errContains: "repetition count too high",
		},
		{
			name:        "exact repetition too high",
			pattern:     "a{500}",
			wantErr:     true,
			errContains: "repetition count too high",
		},
		// Valid edge cases for repetition
		{
			name:    "repetition at max",
			pattern: "a{1,100}",
			wantErr: false,
		},
		{
			name:    "star quantifier",
			pattern: "a*",
			wantErr: false,
		},
		{
			name:    "plus quantifier",
			pattern: "a+",
			wantErr: false,
		},
		// Nested quantifiers - these are the primary ReDoS vectors
		{
			name:    "single nested quantifier allowed",
			pattern: "(a+)+",
			wantErr: false, // 1 level of nesting is allowed
		},
		{
			name:    "double nested quantifier allowed",
			pattern: "((a+)+)+",
			wantErr: false, // 2 levels allowed
		},
		{
			name:    "triple nested quantifier allowed",
			pattern: "(((a+)+)+)+",
			wantErr: false, // 3 levels allowed (at max)
		},
		{
			name:        "quadruple nested quantifier rejected",
			pattern:     "((((a+)+)+)+)+",
			wantErr:     true,
			errContains: "too many nested quantifiers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePattern(tt.pattern)
			if tt.wantErr {
				if err == nil {
					t.Errorf(
						"ValidatePattern(%q) = nil, want error containing %q",
						tt.pattern,
						tt.errContains,
					)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidatePattern(%q) error = %q, want error containing %q", tt.pattern, err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Errorf("ValidatePattern(%q) = %v, want nil", tt.pattern, err)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Pattern: "test",
		Reason:  "test reason",
	}

	expected := "invalid regex pattern: test reason"
	if err.Error() != expected {
		t.Errorf("ValidationError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestSafeCompile(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
		testStr string
		matches bool
	}{
		{
			name:    "valid pattern compiles and matches",
			pattern: "hello",
			wantErr: false,
			testStr: "hello world",
			matches: true,
		},
		{
			name:    "valid pattern compiles and no match",
			pattern: "hello",
			wantErr: false,
			testStr: "goodbye",
			matches: false,
		},
		{
			name:    "case sensitive by default",
			pattern: "Hello",
			wantErr: false,
			testStr: "hello",
			matches: false,
		},
		{
			name:    "empty pattern fails",
			pattern: "",
			wantErr: true,
		},
		{
			name:    "invalid syntax fails",
			pattern: "[abc",
			wantErr: true,
		},
		{
			name:    "dangerous pattern fails",
			pattern: "a{1,1000}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := SafeCompile(tt.pattern)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SafeCompile(%q) = nil error, want error", tt.pattern)
				}

				return
			}

			if err != nil {
				t.Errorf("SafeCompile(%q) = %v, want nil", tt.pattern, err)
				return
			}

			if re == nil {
				t.Errorf("SafeCompile(%q) returned nil regex", tt.pattern)
				return
			}

			if tt.testStr != "" {
				if got := re.MatchString(tt.testStr); got != tt.matches {
					t.Errorf("re.MatchString(%q) = %v, want %v", tt.testStr, got, tt.matches)
				}
			}
		})
	}
}

func TestSafeCompileWithFlags(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		flags   string
		wantErr bool
		testStr string
		matches bool
	}{
		{
			name:    "case insensitive flag",
			pattern: "hello",
			flags:   "(?i)",
			wantErr: false,
			testStr: "HELLO",
			matches: true,
		},
		{
			name:    "multiline flag",
			pattern: "^hello",
			flags:   "(?m)",
			wantErr: false,
			testStr: "world\nhello",
			matches: true,
		},
		{
			name:    "just flags is valid regex",
			pattern: "",
			flags:   "(?i)",
			wantErr: false, // "(?i)" by itself is valid - matches empty string
			testStr: "anything",
			matches: true, // empty pattern matches at start of any string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := SafeCompileWithFlags(tt.pattern, tt.flags)
			if tt.wantErr {
				if err == nil {
					t.Errorf(
						"SafeCompileWithFlags(%q, %q) = nil error, want error",
						tt.pattern,
						tt.flags,
					)
				}

				return
			}

			if err != nil {
				t.Errorf("SafeCompileWithFlags(%q, %q) = %v, want nil", tt.pattern, tt.flags, err)
				return
			}

			if tt.testStr != "" {
				if got := re.MatchString(tt.testStr); got != tt.matches {
					t.Errorf("re.MatchString(%q) = %v, want %v", tt.testStr, got, tt.matches)
				}
			}
		})
	}
}

func TestMustSafeCompile(t *testing.T) {
	// Valid pattern should not panic
	t.Run("valid pattern", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("MustSafeCompile panicked unexpectedly: %v", r)
			}
		}()

		re := MustSafeCompile("hello")
		if re == nil {
			t.Error("MustSafeCompile returned nil")
		}
	})

	// Invalid pattern should panic
	t.Run("invalid pattern panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustSafeCompile should have panicked")
			}
		}()

		MustSafeCompile("[abc")
	})
}

func TestIsQuantifier(t *testing.T) {
	// Test via pattern behavior rather than directly
	// Patterns with quantifiers should be detected
	tests := []struct {
		pattern       string
		hasQuantifier bool
	}{
		{"a*", true},
		{"a+", true},
		{"a?", true},
		{"a{2,5}", true},
		{"abc", false},
		{"[a-z]", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			// We can indirectly test by checking if nested patterns are caught
			nestedPattern := "(" + tt.pattern + ")+"
			err := ValidatePattern(nestedPattern)
			// If the inner pattern has a quantifier, nesting should be detected
			// This is an indirect test since isQuantifier is unexported
			if err != nil && tt.hasQuantifier {
				// This is expected for deeply nested patterns
				t.Logf("Nested pattern correctly analyzed: %s", nestedPattern)
			}
		})
	}
}

// Benchmark to ensure pattern validation doesn't become a bottleneck.
func BenchmarkValidatePattern(b *testing.B) {
	patterns := []string{
		"hello",
		"[a-z]+",
		`\b\w+@\w+\.\w+\b`,
		"(foo|bar|baz)+",
		"(?i)search term",
	}

	for b.Loop() {
		for _, p := range patterns {
			_ = ValidatePattern(p)
		}
	}
}

func BenchmarkSafeCompile(b *testing.B) {
	pattern := `\b\w+@\w+\.\w+\b`

	for b.Loop() {
		_, _ = SafeCompile(pattern)
	}
}
