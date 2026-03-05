// Package files provides file browsing and content retrieval for Git repositories.
package files

import (
	"regexp"
	"sort"
	"strings"
)

// Symbol patterns for various languages.
var (
	// Go patterns.
	goFuncPattern = regexp.MustCompile(`(?m)^func\s+(\([^)]+\)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	goTypePattern = regexp.MustCompile(
		`(?m)^type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(struct|interface)`,
	)
	goMethodPattern = regexp.MustCompile(`(?m)^func\s+\(([^)]+)\)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	// Single-line const/var at package level (no indent).
	goConstPattern = regexp.MustCompile(`^const\s+([A-Za-z_][A-Za-z0-9_]*)`)
	goVarPattern   = regexp.MustCompile(`^var\s+([A-Za-z_][A-Za-z0-9_]*)`)
	// Block detection.
	goConstBlockStart = regexp.MustCompile(`^const\s*\(\s*$`)
	goVarBlockStart   = regexp.MustCompile(`^var\s*\(\s*$`)
	goBlockItem       = regexp.MustCompile(`^\t([A-Za-z_][A-Za-z0-9_]*)`)

	// JavaScript/TypeScript patterns.
	jsFuncPattern = regexp.MustCompile(
		`(?m)^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`,
	)
	jsArrowPattern = regexp.MustCompile(
		`(?m)^(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s+)?\(?[^)]*\)?\s*=>`,
	)
	jsClassPattern = regexp.MustCompile(
		`(?m)^(?:export\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][A-Za-z0-9_$]*)`,
	)
	jsMethodPattern = regexp.MustCompile(
		`(?m)^\s+(?:async\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\s*\([^)]*\)\s*[{:]`,
	)
	jsInterfacePattern = regexp.MustCompile(
		`(?m)^(?:export\s+)?interface\s+([A-Za-z_$][A-Za-z0-9_$]*)`,
	)
	jsTypePattern = regexp.MustCompile(
		`(?m)^(?:export\s+)?type\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=`,
	)
	jsEnumPattern = regexp.MustCompile(`(?m)^(?:export\s+)?enum\s+([A-Za-z_$][A-Za-z0-9_$]*)`)

	// Python patterns.
	pyFuncPattern  = regexp.MustCompile(`(?m)^(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pyClassPattern = regexp.MustCompile(`(?m)^class\s+([A-Za-z_][A-Za-z0-9_]*)`)

	// Java/Kotlin patterns.
	javaClassPattern = regexp.MustCompile(
		`(?m)^(?:public\s+|private\s+|protected\s+)?(?:abstract\s+|final\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`,
	)
	javaMethodPattern = regexp.MustCompile(
		`(?m)^\s+(?:public\s+|private\s+|protected\s+)?(?:static\s+)?(?:final\s+)?(?:async\s+)?(?:[A-Za-z_<>\[\]?,\s]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
	)
	javaInterfacePattern = regexp.MustCompile(
		`(?m)^(?:public\s+|private\s+)?interface\s+([A-Za-z_][A-Za-z0-9_]*)`,
	)
	javaEnumPattern = regexp.MustCompile(
		`(?m)^(?:public\s+|private\s+)?enum\s+([A-Za-z_][A-Za-z0-9_]*)`,
	)

	// Rust patterns.
	rustFnPattern = regexp.MustCompile(
		`(?m)^(?:pub\s+)?(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*[<(]`,
	)
	rustStructPattern = regexp.MustCompile(`(?m)^(?:pub\s+)?struct\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustEnumPattern   = regexp.MustCompile(`(?m)^(?:pub\s+)?enum\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustTraitPattern  = regexp.MustCompile(`(?m)^(?:pub\s+)?trait\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustImplPattern   = regexp.MustCompile(`(?m)^impl(?:<[^>]+>)?\s+([A-Za-z_][A-Za-z0-9_<>]*)`)

	// C/C++ patterns.
	cFuncPattern = regexp.MustCompile(
		`(?m)^(?:[A-Za-z_][A-Za-z0-9_*\s]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^;]*\)\s*{`,
	)
	cStructPattern = regexp.MustCompile(`(?m)^(?:typedef\s+)?struct\s+([A-Za-z_][A-Za-z0-9_]*)`)
	cClassPattern  = regexp.MustCompile(`(?m)^class\s+([A-Za-z_][A-Za-z0-9_]*)`)

	// Ruby patterns.
	rubyMethodPattern = regexp.MustCompile(`(?m)^\s*def\s+([A-Za-z_][A-Za-z0-9_?!]*)`)
	rubyClassPattern  = regexp.MustCompile(`(?m)^class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rubyModulePattern = regexp.MustCompile(`(?m)^module\s+([A-Za-z_][A-Za-z0-9_]*)`)

	// PHP patterns.
	phpFuncPattern = regexp.MustCompile(
		`(?m)^(?:public\s+|private\s+|protected\s+)?(?:static\s+)?function\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
	)
	phpClassPattern = regexp.MustCompile(
		`(?m)^(?:abstract\s+|final\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`,
	)
)

// ExtractSymbols extracts symbols from file content based on language.
func ExtractSymbols(content, language string) []FileSymbol {
	lines := strings.Split(content, "\n")

	var symbols []FileSymbol

	switch strings.ToLower(language) {
	case "go":
		symbols = extractGoSymbols(lines)
	case "javascript", "typescript", "jsx", "tsx":
		symbols = extractJSSymbols(lines)
	case "python":
		symbols = extractPythonSymbols(lines)
	case "java", "kotlin":
		symbols = extractJavaSymbols(lines)
	case "rust":
		symbols = extractRustSymbols(lines)
	case "c", "c++", "cpp":
		symbols = extractCSymbols(lines)
	case "ruby":
		symbols = extractRubySymbols(lines)
	case "php", "hack":
		symbols = extractPHPSymbols(lines)
	default:
		// Try generic function detection for unknown languages
		symbols = extractGenericSymbols(lines)
	}

	// Sort by line number
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Line < symbols[j].Line
	})

	return symbols
}

func extractGoSymbols(lines []string) []FileSymbol {
	var symbols []FileSymbol

	inVarBlock := false
	inConstBlock := false

	for i, line := range lines {
		lineNum := i + 1

		// Check for block starts
		if goVarBlockStart.MatchString(line) {
			inVarBlock = true
			continue
		}

		if goConstBlockStart.MatchString(line) {
			inConstBlock = true
			continue
		}

		// Check for block end
		if (inVarBlock || inConstBlock) && strings.HasPrefix(line, ")") {
			inVarBlock = false
			inConstBlock = false

			continue
		}

		// Handle items inside blocks
		if inVarBlock {
			if matches := goBlockItem.FindStringSubmatch(line); len(matches) > 0 {
				symbols = append(symbols, FileSymbol{
					Name:   matches[1],
					Kind:   "variable",
					Line:   lineNum,
					Column: strings.Index(line, matches[1]) + 1,
				})
			}

			continue
		}

		if inConstBlock {
			if matches := goBlockItem.FindStringSubmatch(line); len(matches) > 0 {
				symbols = append(symbols, FileSymbol{
					Name:   matches[1],
					Kind:   "constant",
					Line:   lineNum,
					Column: strings.Index(line, matches[1]) + 1,
				})
			}

			continue
		}

		// Check for types (struct/interface)
		if matches := goTypePattern.FindStringSubmatch(line); len(matches) > 0 {
			kind := "struct"
			if matches[2] == "interface" {
				kind = "interface"
			}

			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      kind,
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for methods (func with receiver)
		if matches := goMethodPattern.FindStringSubmatch(line); len(matches) > 0 {
			receiver := strings.TrimSpace(matches[1])
			parts := strings.Fields(receiver)

			parent := ""
			if len(parts) > 0 {
				parent = strings.TrimPrefix(parts[len(parts)-1], "*")
			}

			symbols = append(symbols, FileSymbol{
				Name:      matches[2],
				Kind:      "method",
				Line:      lineNum,
				Column:    strings.Index(line, matches[2]) + 1,
				Signature: strings.TrimSpace(line),
				Parent:    parent,
			})

			continue
		}

		// Check for functions
		if matches := goFuncPattern.FindStringSubmatch(line); len(matches) > 0 && matches[1] == "" {
			symbols = append(symbols, FileSymbol{
				Name:      matches[2],
				Kind:      "function",
				Line:      lineNum,
				Column:    strings.Index(line, matches[2]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for single-line const
		if matches := goConstPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:   matches[1],
				Kind:   "constant",
				Line:   lineNum,
				Column: strings.Index(line, matches[1]) + 1,
			})

			continue
		}

		// Check for single-line var
		if matches := goVarPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:   matches[1],
				Kind:   "variable",
				Line:   lineNum,
				Column: strings.Index(line, matches[1]) + 1,
			})
		}
	}

	return symbols
}

func extractJSSymbols(lines []string) []FileSymbol {
	var (
		symbols      []FileSymbol
		currentClass string
	)

	for i, line := range lines {
		lineNum := i + 1

		// Check for classes
		if matches := jsClassPattern.FindStringSubmatch(line); len(matches) > 0 {
			currentClass = matches[1]
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "class",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for interfaces (TypeScript)
		if matches := jsInterfacePattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "interface",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for type aliases (TypeScript)
		if matches := jsTypePattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "type",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for enums (TypeScript)
		if matches := jsEnumPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "enum",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for functions
		if matches := jsFuncPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "function",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for arrow functions
		if matches := jsArrowPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "function",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for class methods (only if inside a class context)
		if currentClass != "" && strings.HasPrefix(strings.TrimSpace(line), "") {
			if matches := jsMethodPattern.FindStringSubmatch(line); len(matches) > 0 {
				// Skip constructor and common non-method patterns
				name := matches[1]
				if name != "if" && name != "for" && name != "while" && name != "switch" &&
					name != "catch" {
					symbols = append(symbols, FileSymbol{
						Name:      name,
						Kind:      "method",
						Line:      lineNum,
						Column:    strings.Index(line, name) + 1,
						Signature: strings.TrimSpace(line),
						Parent:    currentClass,
					})
				}
			}
		}

		// Reset class context on closing brace at start of line
		if strings.TrimSpace(line) == "}" {
			currentClass = ""
		}
	}

	return symbols
}

func extractPythonSymbols(lines []string) []FileSymbol {
	var (
		symbols      []FileSymbol
		currentClass string
		classIndent  int
	)

	for i, line := range lines {
		lineNum := i + 1

		// Track indentation
		trimmed := strings.TrimLeft(line, " \t")
		indent := len(line) - len(trimmed)

		// Check for class definitions
		if matches := pyClassPattern.FindStringSubmatch(line); len(matches) > 0 {
			currentClass = matches[1]
			classIndent = indent

			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "class",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for function/method definitions
		if matches := pyFuncPattern.FindStringSubmatch(line); len(matches) > 0 {
			kind := "function"
			parent := ""

			// If indented more than a class, it's a method
			if currentClass != "" && indent > classIndent {
				kind = "method"
				parent = currentClass
			}

			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      kind,
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
				Parent:    parent,
			})

			continue
		}

		// Reset class context when we see a non-indented non-empty line
		if indent <= classIndent && trimmed != "" && currentClass != "" {
			if !strings.HasPrefix(trimmed, "def ") && !strings.HasPrefix(trimmed, "@") {
				currentClass = ""
			}
		}
	}

	return symbols
}

func extractJavaSymbols(lines []string) []FileSymbol {
	var (
		symbols      []FileSymbol
		currentClass string
	)

	for i, line := range lines {
		lineNum := i + 1

		// Check for class definitions
		if matches := javaClassPattern.FindStringSubmatch(line); len(matches) > 0 {
			currentClass = matches[1]
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "class",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for interfaces
		if matches := javaInterfacePattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "interface",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for enums
		if matches := javaEnumPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "enum",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for methods
		if matches := javaMethodPattern.FindStringSubmatch(line); len(matches) > 0 {
			name := matches[1]
			// Skip common keywords
			if name != "if" && name != "for" && name != "while" && name != "switch" &&
				name != "catch" &&
				name != "return" {
				symbols = append(symbols, FileSymbol{
					Name:      name,
					Kind:      "method",
					Line:      lineNum,
					Column:    strings.Index(line, name) + 1,
					Signature: strings.TrimSpace(line),
					Parent:    currentClass,
				})
			}
		}
	}

	return symbols
}

func extractRustSymbols(lines []string) []FileSymbol {
	var (
		symbols     []FileSymbol
		currentImpl string
	)

	for i, line := range lines {
		lineNum := i + 1

		// Check for struct definitions
		if matches := rustStructPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "struct",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for enum definitions
		if matches := rustEnumPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "enum",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for trait definitions
		if matches := rustTraitPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "trait",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for impl blocks
		if matches := rustImplPattern.FindStringSubmatch(line); len(matches) > 0 {
			currentImpl = matches[1]
			continue
		}

		// Check for functions
		if matches := rustFnPattern.FindStringSubmatch(line); len(matches) > 0 {
			kind := "function"
			parent := ""

			if currentImpl != "" && strings.HasPrefix(line, "    ") {
				kind = "method"
				parent = currentImpl
			}

			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      kind,
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
				Parent:    parent,
			})
		}

		// Reset impl context on closing brace
		if strings.TrimSpace(line) == "}" {
			currentImpl = ""
		}
	}

	return symbols
}

func extractCSymbols(lines []string) []FileSymbol {
	var symbols []FileSymbol

	for i, line := range lines {
		lineNum := i + 1

		// Check for class definitions (C++)
		if matches := cClassPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "class",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for struct definitions
		if matches := cStructPattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "struct",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for function definitions
		if matches := cFuncPattern.FindStringSubmatch(line); len(matches) > 0 {
			name := matches[1]
			if name != "if" && name != "for" && name != "while" && name != "switch" {
				symbols = append(symbols, FileSymbol{
					Name:      name,
					Kind:      "function",
					Line:      lineNum,
					Column:    strings.Index(line, name) + 1,
					Signature: strings.TrimSpace(line),
				})
			}
		}
	}

	return symbols
}

func extractRubySymbols(lines []string) []FileSymbol {
	var (
		symbols      []FileSymbol
		currentClass string
	)

	for i, line := range lines {
		lineNum := i + 1

		// Check for module definitions
		if matches := rubyModulePattern.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "module",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for class definitions
		if matches := rubyClassPattern.FindStringSubmatch(line); len(matches) > 0 {
			currentClass = matches[1]
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "class",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for method definitions
		if matches := rubyMethodPattern.FindStringSubmatch(line); len(matches) > 0 {
			kind := "function"
			parent := ""

			if currentClass != "" && strings.HasPrefix(line, "  ") {
				kind = "method"
				parent = currentClass
			}

			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      kind,
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
				Parent:    parent,
			})
		}

		// Reset class context on 'end' at start of line
		if strings.TrimSpace(line) == "end" {
			currentClass = ""
		}
	}

	return symbols
}

func extractPHPSymbols(lines []string) []FileSymbol {
	var (
		symbols      []FileSymbol
		currentClass string
	)

	for i, line := range lines {
		lineNum := i + 1

		// Check for class definitions
		if matches := phpClassPattern.FindStringSubmatch(line); len(matches) > 0 {
			currentClass = matches[1]
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "class",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})

			continue
		}

		// Check for function definitions
		if matches := phpFuncPattern.FindStringSubmatch(line); len(matches) > 0 {
			kind := "function"
			parent := ""

			if currentClass != "" && strings.HasPrefix(line, "    ") {
				kind = "method"
				parent = currentClass
			}

			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      kind,
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
				Parent:    parent,
			})
		}
	}

	return symbols
}

func extractGenericSymbols(lines []string) []FileSymbol {
	// For unknown languages, try to detect function-like patterns
	var symbols []FileSymbol

	genericFunc := regexp.MustCompile(
		`(?m)^(?:func|function|def|fn|sub)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`,
	)

	for i, line := range lines {
		lineNum := i + 1

		if matches := genericFunc.FindStringSubmatch(line); len(matches) > 0 {
			symbols = append(symbols, FileSymbol{
				Name:      matches[1],
				Kind:      "function",
				Line:      lineNum,
				Column:    strings.Index(line, matches[1]) + 1,
				Signature: strings.TrimSpace(line),
			})
		}
	}

	return symbols
}
