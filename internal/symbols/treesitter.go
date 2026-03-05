// Package symbols provides symbol extraction using Tree-sitter for accurate AST parsing.
package symbols

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
)

// Symbol represents a code symbol (function, class, method, etc.)
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"` // function, class, method, struct, interface, etc.
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"endLine,omitempty"`
	EndColumn int    `json:"endColumn,omitempty"`
	Signature string `json:"signature,omitempty"`
	Parent    string `json:"parent,omitempty"`
	FilePath  string `json:"filePath,omitempty"`
}

// TreeSitterService provides AST-based symbol extraction.
type TreeSitterService struct {
	parsers map[string]*sitter.Language
}

// NewTreeSitterService creates a new Tree-sitter service with supported languages.
func NewTreeSitterService() *TreeSitterService {
	return &TreeSitterService{
		parsers: map[string]*sitter.Language{
			"go":         golang.GetLanguage(),
			"javascript": javascript.GetLanguage(),
			"typescript": javascript.GetLanguage(), // JS parser handles TS reasonably well
			"jsx":        javascript.GetLanguage(),
			"tsx":        javascript.GetLanguage(),
			"python":     python.GetLanguage(),
			"java":       java.GetLanguage(),
			"rust":       rust.GetLanguage(),
			"php":        php.GetLanguage(),
			"hack":       php.GetLanguage(), // PHP parser handles Hack reasonably well
			"hcl":        hcl.GetLanguage(),
		},
	}
}

// SupportedLanguages returns the list of languages with Tree-sitter support.
func (s *TreeSitterService) SupportedLanguages() []string {
	langs := make([]string, 0, len(s.parsers))
	for lang := range s.parsers {
		langs = append(langs, lang)
	}

	return langs
}

// IsSupported checks if a language has Tree-sitter support.
func (s *TreeSitterService) IsSupported(lang string) bool {
	_, ok := s.parsers[strings.ToLower(lang)]
	return ok
}

// ExtractSymbols extracts symbols from source code using Tree-sitter.
func (s *TreeSitterService) ExtractSymbols(
	ctx context.Context,
	code []byte,
	lang string,
) ([]Symbol, error) {
	langKey := strings.ToLower(lang)

	language, ok := s.parsers[langKey]
	if !ok {
		return nil, nil // Return empty for unsupported languages
	}

	parser := sitter.NewParser()
	parser.SetLanguage(language)

	tree, err := parser.ParseCtx(ctx, nil, code)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	rootNode := tree.RootNode()

	var symbols []Symbol

	switch langKey {
	case "go":
		symbols = s.extractGoSymbols(rootNode, code)
	case "javascript", "typescript", "jsx", "tsx":
		symbols = s.extractJSSymbols(rootNode, code)
	case "python":
		symbols = s.extractPythonSymbols(rootNode, code)
	case "java":
		symbols = s.extractJavaSymbols(rootNode, code)
	case "rust":
		symbols = s.extractRustSymbols(rootNode, code)
	case "php", "hack":
		symbols = s.extractPHPSymbols(rootNode, code)
	case "hcl":
		symbols = s.extractHCLSymbols(rootNode, code)
	}

	return symbols, nil
}

// GetSymbolAtPosition returns the symbol at a specific line and column.
func (s *TreeSitterService) GetSymbolAtPosition(
	ctx context.Context,
	code []byte,
	lang string,
	line, col int,
) (*Symbol, error) {
	symbols, err := s.ExtractSymbols(ctx, code, lang)
	if err != nil {
		return nil, err
	}

	// Find the most specific symbol containing the position
	var best *Symbol

	for i := range symbols {
		sym := &symbols[i]
		if sym.Line <= line && (sym.EndLine == 0 || sym.EndLine >= line) {
			if best == nil || sym.Line > best.Line {
				best = sym
			}
		}
	}

	return best, nil
}

// extractGoSymbols extracts symbols from Go source code.
func (s *TreeSitterService) extractGoSymbols(node *sitter.Node, code []byte) []Symbol {
	var symbols []Symbol

	s.walkNode(node, code, "", func(n *sitter.Node, parent string) {
		switch n.Type() {
		case "function_declaration":
			if name := s.findChildByType(n, "identifier"); name != nil {
				sig := s.getNodeText(n, code)
				// Truncate signature to first line
				if idx := strings.Index(sig, "{"); idx > 0 {
					sig = strings.TrimSpace(sig[:idx])
				}

				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "function",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: sig,
				})
			}

		case "method_declaration":
			if name := s.findChildByType(n, "field_identifier"); name != nil {
				// Get receiver type
				receiver := ""

				if params := s.findChildByType(n, "parameter_list"); params != nil {
					if params.ChildCount() > 0 {
						param := params.Child(0)
						if typeNode := s.findChildByType(param, "type_identifier"); typeNode != nil {
							receiver = s.getNodeText(typeNode, code)
						} else if ptr := s.findChildByType(param, "pointer_type"); ptr != nil {
							if typeNode := s.findChildByType(ptr, "type_identifier"); typeNode != nil {
								receiver = s.getNodeText(typeNode, code)
							}
						}
					}
				}

				sig := s.getNodeText(n, code)
				if idx := strings.Index(sig, "{"); idx > 0 {
					sig = strings.TrimSpace(sig[:idx])
				}

				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "method",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: sig,
					Parent:    receiver,
				})
			}

		case "type_declaration":
			// Look for type_spec children
			for i := range n.NamedChildCount() {
				child := n.NamedChild(int(i))
				if child.Type() == "type_spec" {
					if name := s.findChildByType(child, "type_identifier"); name != nil {
						kind := "type"
						if s.findChildByType(child, "struct_type") != nil {
							kind = "struct"
						} else if s.findChildByType(child, "interface_type") != nil {
							kind = "interface"
						}

						symbols = append(symbols, Symbol{
							Name:      s.getNodeText(name, code),
							Kind:      kind,
							Line:      int(child.StartPoint().Row) + 1,
							Column:    int(child.StartPoint().Column) + 1,
							EndLine:   int(child.EndPoint().Row) + 1,
							EndColumn: int(child.EndPoint().Column) + 1,
							Signature: s.getFirstLine(s.getNodeText(child, code)),
						})
					}
				}
			}

		case "const_declaration", "var_declaration":
			// Only extract top-level const/var declarations (parent must be source_file)
			if n.Parent() == nil || n.Parent().Type() != "source_file" {
				return // Skip non-top-level declarations (e.g., inside functions)
			}

			kind := "variable"
			if n.Type() == "const_declaration" {
				kind = "constant"
			}

			for i := range n.NamedChildCount() {
				child := n.NamedChild(int(i))
				if child.Type() == "const_spec" || child.Type() == "var_spec" {
					if name := s.findChildByType(child, "identifier"); name != nil {
						symbols = append(symbols, Symbol{
							Name:   s.getNodeText(name, code),
							Kind:   kind,
							Line:   int(child.StartPoint().Row) + 1,
							Column: int(child.StartPoint().Column) + 1,
						})
					}
				}
			}
		}
	})

	return symbols
}

// extractJSSymbols extracts symbols from JavaScript/TypeScript source code.
func (s *TreeSitterService) extractJSSymbols(node *sitter.Node, code []byte) []Symbol {
	var (
		symbols    []Symbol
		classStack []string
	)

	s.walkNodeWithStack(node, code, func(n *sitter.Node, entering bool) {
		if !entering {
			// Pop class from stack when leaving
			if n.Type() == "class_declaration" || n.Type() == "class" {
				if len(classStack) > 0 {
					classStack = classStack[:len(classStack)-1]
				}
			}

			return
		}

		currentClass := ""
		if len(classStack) > 0 {
			currentClass = classStack[len(classStack)-1]
		}

		switch n.Type() {
		case "class_declaration", "class":
			if name := s.findChildByType(n, "identifier"); name != nil {
				className := s.getNodeText(name, code)
				classStack = append(classStack, className)
				symbols = append(symbols, Symbol{
					Name:      className,
					Kind:      "class",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "function_declaration":
			if name := s.findChildByType(n, "identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "function",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "method_definition":
			if name := s.findChildByType(n, "property_identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "method",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
					Parent:    currentClass,
				})
			}

		case "lexical_declaration", "variable_declaration":
			// Arrow functions and const/let declarations
			for i := range n.NamedChildCount() {
				child := n.NamedChild(int(i))
				if child.Type() == "variable_declarator" {
					if name := s.findChildByType(child, "identifier"); name != nil {
						// Check if it's an arrow function
						if arrow := s.findChildByType(child, "arrow_function"); arrow != nil {
							symbols = append(symbols, Symbol{
								Name:      s.getNodeText(name, code),
								Kind:      "function",
								Line:      int(child.StartPoint().Row) + 1,
								Column:    int(child.StartPoint().Column) + 1,
								EndLine:   int(child.EndPoint().Row) + 1,
								EndColumn: int(child.EndPoint().Column) + 1,
								Signature: s.getFirstLine(s.getNodeText(child, code)),
							})
						}
					}
				}
			}

		case "interface_declaration":
			if name := s.findChildByType(n, "type_identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "interface",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "type_alias_declaration":
			if name := s.findChildByType(n, "type_identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "type",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "enum_declaration":
			if name := s.findChildByType(n, "identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "enum",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}
		}
	})

	return symbols
}

// extractPythonSymbols extracts symbols from Python source code.
func (s *TreeSitterService) extractPythonSymbols(node *sitter.Node, code []byte) []Symbol {
	var (
		symbols    []Symbol
		classStack []string
	)

	s.walkNodeWithStack(node, code, func(n *sitter.Node, entering bool) {
		if !entering {
			if n.Type() == "class_definition" {
				if len(classStack) > 0 {
					classStack = classStack[:len(classStack)-1]
				}
			}

			return
		}

		currentClass := ""
		if len(classStack) > 0 {
			currentClass = classStack[len(classStack)-1]
		}

		switch n.Type() {
		case "class_definition":
			if name := s.findChildByType(n, "identifier"); name != nil {
				className := s.getNodeText(name, code)
				classStack = append(classStack, className)
				symbols = append(symbols, Symbol{
					Name:      className,
					Kind:      "class",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "function_definition":
			if name := s.findChildByType(n, "identifier"); name != nil {
				kind := "function"
				if currentClass != "" {
					kind = "method"
				}

				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      kind,
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
					Parent:    currentClass,
				})
			}
		}
	})

	return symbols
}

// extractJavaSymbols extracts symbols from Java source code.
func (s *TreeSitterService) extractJavaSymbols(node *sitter.Node, code []byte) []Symbol {
	var (
		symbols    []Symbol
		classStack []string
	)

	s.walkNodeWithStack(node, code, func(n *sitter.Node, entering bool) {
		if !entering {
			if n.Type() == "class_declaration" || n.Type() == "interface_declaration" {
				if len(classStack) > 0 {
					classStack = classStack[:len(classStack)-1]
				}
			}

			return
		}

		currentClass := ""
		if len(classStack) > 0 {
			currentClass = classStack[len(classStack)-1]
		}

		switch n.Type() {
		case "class_declaration":
			if name := s.findChildByType(n, "identifier"); name != nil {
				className := s.getNodeText(name, code)
				classStack = append(classStack, className)
				symbols = append(symbols, Symbol{
					Name:      className,
					Kind:      "class",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "interface_declaration":
			if name := s.findChildByType(n, "identifier"); name != nil {
				className := s.getNodeText(name, code)
				classStack = append(classStack, className)
				symbols = append(symbols, Symbol{
					Name:      className,
					Kind:      "interface",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "enum_declaration":
			if name := s.findChildByType(n, "identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "enum",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "method_declaration":
			if name := s.findChildByType(n, "identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "method",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
					Parent:    currentClass,
				})
			}

		case "constructor_declaration":
			if name := s.findChildByType(n, "identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "constructor",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
					Parent:    currentClass,
				})
			}
		}
	})

	return symbols
}

// extractRustSymbols extracts symbols from Rust source code.
func (s *TreeSitterService) extractRustSymbols(node *sitter.Node, code []byte) []Symbol {
	var (
		symbols   []Symbol
		implStack []string
	)

	s.walkNodeWithStack(node, code, func(n *sitter.Node, entering bool) {
		if !entering {
			if n.Type() == "impl_item" {
				if len(implStack) > 0 {
					implStack = implStack[:len(implStack)-1]
				}
			}

			return
		}

		currentImpl := ""
		if len(implStack) > 0 {
			currentImpl = implStack[len(implStack)-1]
		}

		switch n.Type() {
		case "struct_item":
			if name := s.findChildByType(n, "type_identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "struct",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "enum_item":
			if name := s.findChildByType(n, "type_identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "enum",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "trait_item":
			if name := s.findChildByType(n, "type_identifier"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "trait",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "impl_item":
			if typeNode := s.findChildByType(n, "type_identifier"); typeNode != nil {
				implStack = append(implStack, s.getNodeText(typeNode, code))
			}

		case "function_item":
			if name := s.findChildByType(n, "identifier"); name != nil {
				kind := "function"
				if currentImpl != "" {
					kind = "method"
				}

				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      kind,
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
					Parent:    currentImpl,
				})
			}
		}
	})

	return symbols
}

// extractPHPSymbols extracts symbols from PHP source code.
func (s *TreeSitterService) extractPHPSymbols(node *sitter.Node, code []byte) []Symbol {
	var (
		symbols    []Symbol
		classStack []string
	)

	s.walkNodeWithStack(node, code, func(n *sitter.Node, entering bool) {
		if !entering {
			if n.Type() == "class_declaration" || n.Type() == "interface_declaration" ||
				n.Type() == "trait_declaration" {
				if len(classStack) > 0 {
					classStack = classStack[:len(classStack)-1]
				}
			}

			return
		}

		currentClass := ""
		if len(classStack) > 0 {
			currentClass = classStack[len(classStack)-1]
		}

		switch n.Type() {
		case "class_declaration":
			if name := s.findChildByType(n, "name"); name != nil {
				className := s.getNodeText(name, code)
				classStack = append(classStack, className)
				symbols = append(symbols, Symbol{
					Name:      className,
					Kind:      "class",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "interface_declaration":
			if name := s.findChildByType(n, "name"); name != nil {
				interfaceName := s.getNodeText(name, code)
				classStack = append(classStack, interfaceName)
				symbols = append(symbols, Symbol{
					Name:      interfaceName,
					Kind:      "interface",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "trait_declaration":
			if name := s.findChildByType(n, "name"); name != nil {
				traitName := s.getNodeText(name, code)
				classStack = append(classStack, traitName)
				symbols = append(symbols, Symbol{
					Name:      traitName,
					Kind:      "trait",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "enum_declaration":
			if name := s.findChildByType(n, "name"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "enum",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "function_definition":
			if name := s.findChildByType(n, "name"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "function",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
				})
			}

		case "method_declaration":
			if name := s.findChildByType(n, "name"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "method",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
					Signature: s.getFirstLine(s.getNodeText(n, code)),
					Parent:    currentClass,
				})
			}

		case "property_declaration":
			// Find all property elements within the declaration
			for i := range n.NamedChildCount() {
				child := n.NamedChild(int(i))
				if child.Type() == "property_element" {
					if varName := s.findChildByType(child, "variable_name"); varName != nil {
						symbols = append(symbols, Symbol{
							Name:      s.getNodeText(varName, code),
							Kind:      "property",
							Line:      int(child.StartPoint().Row) + 1,
							Column:    int(child.StartPoint().Column) + 1,
							EndLine:   int(child.EndPoint().Row) + 1,
							EndColumn: int(child.EndPoint().Column) + 1,
							Parent:    currentClass,
						})
					}
				}
			}

		case "const_declaration":
			// Find all const elements within the declaration
			for i := range n.NamedChildCount() {
				child := n.NamedChild(int(i))
				if child.Type() == "const_element" {
					if name := s.findChildByType(child, "name"); name != nil {
						symbols = append(symbols, Symbol{
							Name:      s.getNodeText(name, code),
							Kind:      "constant",
							Line:      int(child.StartPoint().Row) + 1,
							Column:    int(child.StartPoint().Column) + 1,
							EndLine:   int(child.EndPoint().Row) + 1,
							EndColumn: int(child.EndPoint().Column) + 1,
							Parent:    currentClass,
						})
					}
				}
			}

		case "namespace_definition":
			if name := s.findChildByType(n, "namespace_name"); name != nil {
				symbols = append(symbols, Symbol{
					Name:      s.getNodeText(name, code),
					Kind:      "namespace",
					Line:      int(n.StartPoint().Row) + 1,
					Column:    int(n.StartPoint().Column) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					EndColumn: int(n.EndPoint().Column) + 1,
				})
			}
		}
	})

	return symbols
}

func (s *TreeSitterService) extractHCLSymbols(node *sitter.Node, code []byte) []Symbol {
	var symbols []Symbol

	s.walkNode(node, code, "", func(n *sitter.Node, parent string) {
		switch n.Type() {
		case "block":
			symbol := s.extractHCLBlock(n, code)
			if symbol != nil {
				symbols = append(symbols, *symbol)
			}

		case "attribute":
			// Top-level attributes (e.g., `terraform_version = "1.0"`)
			if parent == "" || parent == "body" {
				if nameNode := s.findChildByType(n, "identifier"); nameNode != nil {
					symbols = append(symbols, Symbol{
						Name:      s.getNodeText(nameNode, code),
						Kind:      "variable",
						Line:      int(n.StartPoint().Row) + 1,
						Column:    int(n.StartPoint().Column) + 1,
						EndLine:   int(n.EndPoint().Row) + 1,
						EndColumn: int(n.EndPoint().Column) + 1,
						Signature: strings.TrimSpace(s.getNodeText(n, code)),
					})
				}
			}
		}
	})

	return symbols
}

func (s *TreeSitterService) extractHCLBlock(n *sitter.Node, code []byte) *Symbol {
	// HCL blocks have structure: block_type label* { body }
	// Example: resource "aws_instance" "web" { ... }
	var (
		blockType string
		labels    []string
	)

	for i := range n.ChildCount() {
		child := n.Child(int(i))
		switch child.Type() {
		case "identifier":
			if blockType == "" {
				blockType = s.getNodeText(child, code)
			}
		case "string_lit":
			// Remove quotes from labels
			label := s.getNodeText(child, code)
			label = strings.Trim(label, `"`)
			labels = append(labels, label)
		}
	}

	if blockType == "" {
		return nil
	}

	// Determine the kind and name based on block type
	kind, name := s.classifyHCLBlock(blockType, labels)

	return &Symbol{
		Name:      name,
		Kind:      kind,
		Line:      int(n.StartPoint().Row) + 1,
		Column:    int(n.StartPoint().Column) + 1,
		EndLine:   int(n.EndPoint().Row) + 1,
		EndColumn: int(n.EndPoint().Column) + 1,
		Signature: s.getFirstLine(s.getNodeText(n, code)),
	}
}

func (s *TreeSitterService) classifyHCLBlock(
	blockType string,
	labels []string,
) (kind string, name string) {
	switch blockType {
	// Terraform block types
	case "resource":
		if len(labels) >= 2 {
			return "resource", labels[0] + "." + labels[1]
		} else if len(labels) == 1 {
			return "resource", labels[0]
		}

		return "resource", blockType

	case "data":
		if len(labels) >= 2 {
			return "data", "data." + labels[0] + "." + labels[1]
		}

		return "data", blockType

	case "variable":
		if len(labels) >= 1 {
			return "variable", "var." + labels[0]
		}

		return "variable", blockType

	case "output":
		if len(labels) >= 1 {
			return "output", labels[0]
		}

		return "output", blockType

	case "module":
		if len(labels) >= 1 {
			return "module", "module." + labels[0]
		}

		return "module", blockType

	case "provider":
		if len(labels) >= 1 {
			return "provider", labels[0]
		}

		return "provider", blockType

	case "locals":
		return "locals", "locals"

	case "terraform":
		return "terraform", "terraform"

	// Nested blocks
	case "provisioner", "connection", "lifecycle", "backend":
		if len(labels) >= 1 {
			return blockType, labels[0]
		}

		return blockType, blockType

	default:
		if len(labels) >= 1 {
			return "block", labels[0]
		}

		return "block", blockType
	}
}

// Helper functions

func (s *TreeSitterService) walkNode(
	node *sitter.Node,
	_ []byte, // code reserved for future use
	parent string,
	fn func(*sitter.Node, string),
) {
	fn(node, parent)

	for i := range node.ChildCount() {
		child := node.Child(int(i))
		s.walkNode(child, nil, parent, fn)
	}
}

func (s *TreeSitterService) walkNodeWithStack(
	node *sitter.Node,
	_ []byte, // code reserved for future use
	fn func(*sitter.Node, bool),
) {
	fn(node, true) // entering

	for i := range node.ChildCount() {
		child := node.Child(int(i))
		s.walkNodeWithStack(child, nil, fn)
	}

	fn(node, false) // leaving
}

func (s *TreeSitterService) findChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	for i := range node.ChildCount() {
		child := node.Child(int(i))
		if child.Type() == nodeType {
			return child
		}
	}

	return nil
}

func (s *TreeSitterService) getNodeText(node *sitter.Node, code []byte) string {
	return string(code[node.StartByte():node.EndByte()])
}

func (s *TreeSitterService) getFirstLine(text string) string {
	if idx := strings.Index(text, "\n"); idx > 0 {
		return strings.TrimSpace(text[:idx])
	}

	return strings.TrimSpace(text)
}
