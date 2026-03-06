package symbols

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewTreeSitterService(t *testing.T) {
	svc := NewTreeSitterService()
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	if len(svc.parsers) == 0 {
		t.Error("expected parsers to be initialized")
	}
}

func TestTreeSitterService_SupportedLanguages(t *testing.T) {
	svc := NewTreeSitterService()
	langs := svc.SupportedLanguages()

	if len(langs) == 0 {
		t.Error("expected at least one supported language")
	}

	langMap := make(map[string]bool)
	for _, l := range langs {
		langMap[l] = true
	}

	expected := []string{"go", "javascript", "python", "java", "rust"}
	for _, lang := range expected {
		if !langMap[lang] {
			t.Errorf("expected %s to be supported", lang)
		}
	}
}

func TestTreeSitterService_IsSupported(t *testing.T) {
	svc := NewTreeSitterService()

	tests := []struct {
		lang     string
		expected bool
	}{
		{"go", true},
		{"Go", true},
		{"GO", true},
		{"javascript", true},
		{"python", true},
		{"java", true},
		{"rust", true},
		{"php", true},
		{"typescript", true},
		{"hcl", true},
		{"cobol", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			got := svc.IsSupported(tt.lang)
			if got != tt.expected {
				t.Errorf("IsSupported(%q) = %v, want %v", tt.lang, got, tt.expected)
			}
		})
	}
}

func TestTreeSitterService_ExtractGoSymbols(t *testing.T) {
	svc := NewTreeSitterService()
	ctx := context.Background()

	code := []byte(`package main

type User struct {
	Name string
	Age  int
}

type Greeter interface {
	Greet() string
}

func NewUser(name string, age int) *User {
	return &User{Name: name, Age: age}
}

func (u *User) Greet() string {
	return "Hello"
}

func main() {
}
`)

	symbols, err := svc.ExtractSymbols(ctx, code, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected symbols to be extracted")
	}

	symbolNames := make(map[string]Symbol)
	for _, s := range symbols {
		symbolNames[s.Name] = s
	}

	if s, ok := symbolNames["User"]; !ok {
		t.Error("expected User struct")
	} else if s.Kind != "struct" {
		t.Errorf("expected User to be struct, got %s", s.Kind)
	}

	if s, ok := symbolNames["Greeter"]; !ok {
		t.Error("expected Greeter interface")
	} else if s.Kind != "interface" {
		t.Errorf("expected Greeter to be interface, got %s", s.Kind)
	}

	if s, ok := symbolNames["NewUser"]; !ok {
		t.Error("expected NewUser function")
	} else if s.Kind != "function" {
		t.Errorf("expected NewUser to be function, got %s", s.Kind)
	}

	if s, ok := symbolNames["Greet"]; !ok {
		t.Error("expected Greet method")
	} else if s.Kind != "method" {
		t.Errorf("expected Greet to be method, got %s", s.Kind)
	}
}

func TestTreeSitterService_ExtractPythonSymbols(t *testing.T) {
	svc := NewTreeSitterService()
	ctx := context.Background()

	code := []byte(`class User:
    def __init__(self, name):
        self.name = name

    def greet(self):
        return "Hello"

def create_user(name):
    return User(name)
`)

	symbols, err := svc.ExtractSymbols(ctx, code, "python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected symbols to be extracted")
	}

	symbolNames := make(map[string]Symbol)
	for _, s := range symbols {
		symbolNames[s.Name] = s
	}

	if s, ok := symbolNames["User"]; !ok {
		t.Error("expected User class")
	} else if s.Kind != "class" {
		t.Errorf("expected User to be class, got %s", s.Kind)
	}

	if s, ok := symbolNames["create_user"]; !ok {
		t.Error("expected create_user function")
	} else if s.Kind != "function" {
		t.Errorf("expected create_user to be function, got %s", s.Kind)
	}
}

func TestTreeSitterService_ExtractJavaScriptSymbols(t *testing.T) {
	svc := NewTreeSitterService()
	ctx := context.Background()

	code := []byte(`class User {
    constructor(name) {
        this.name = name;
    }
}

function createUser(name) {
    return new User(name);
}
`)

	symbols, err := svc.ExtractSymbols(ctx, code, "javascript")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected symbols to be extracted")
	}

	symbolNames := make(map[string]Symbol)
	for _, s := range symbols {
		symbolNames[s.Name] = s
	}

	if s, ok := symbolNames["User"]; !ok {
		t.Error("expected User class")
	} else if s.Kind != "class" {
		t.Errorf("expected User to be class, got %s", s.Kind)
	}

	if s, ok := symbolNames["createUser"]; !ok {
		t.Error("expected createUser function")
	} else if s.Kind != "function" {
		t.Errorf("expected createUser to be function, got %s", s.Kind)
	}
}

func TestTreeSitterService_ExtractJavaSymbols(t *testing.T) {
	svc := NewTreeSitterService()
	ctx := context.Background()

	code := []byte(`public class User {
    private String name;

    public User(String name) {
        this.name = name;
    }
}

interface Greeter {
    String greet();
}
`)

	symbols, err := svc.ExtractSymbols(ctx, code, "java")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected symbols to be extracted")
	}

	symbolNames := make(map[string]Symbol)
	for _, s := range symbols {
		symbolNames[s.Name] = s
	}

	// User might be parsed as constructor in some cases
	if s, ok := symbolNames["User"]; !ok {
		t.Error("expected User symbol")
	} else if s.Kind != "class" && s.Kind != "constructor" {
		t.Errorf("expected User to be class or constructor, got %s", s.Kind)
	}

	if s, ok := symbolNames["Greeter"]; !ok {
		t.Error("expected Greeter interface")
	} else if s.Kind != "interface" {
		t.Errorf("expected Greeter to be interface, got %s", s.Kind)
	}
}

func TestTreeSitterService_ExtractRustSymbols(t *testing.T) {
	svc := NewTreeSitterService()
	ctx := context.Background()

	code := []byte(`struct User {
    name: String,
}

impl User {
    fn new(name: String) -> Self {
        User { name }
    }
}

trait Greeter {
    fn greet(&self) -> String;
}

fn create_user(name: &str) -> User {
    User::new(name.to_string())
}
`)

	symbols, err := svc.ExtractSymbols(ctx, code, "rust")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected symbols to be extracted")
	}

	symbolNames := make(map[string]Symbol)
	for _, s := range symbols {
		symbolNames[s.Name] = s
	}

	if s, ok := symbolNames["User"]; !ok {
		t.Error("expected User struct")
	} else if s.Kind != "struct" {
		t.Errorf("expected User to be struct, got %s", s.Kind)
	}

	if s, ok := symbolNames["Greeter"]; !ok {
		t.Error("expected Greeter trait")
	} else if s.Kind != "trait" {
		t.Errorf("expected Greeter to be trait, got %s", s.Kind)
	}

	if s, ok := symbolNames["create_user"]; !ok {
		t.Error("expected create_user function")
	} else if s.Kind != "function" {
		t.Errorf("expected create_user to be function, got %s", s.Kind)
	}
}

func TestTreeSitterService_UnsupportedLanguage(t *testing.T) {
	svc := NewTreeSitterService()
	ctx := context.Background()

	code := []byte(`(defun hello () (print "Hello"))`)

	symbols, err := svc.ExtractSymbols(ctx, code, "lisp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(symbols) > 0 {
		t.Errorf("expected empty symbols for unsupported language, got %d", len(symbols))
	}
}

func TestTreeSitterService_EmptyCode(t *testing.T) {
	svc := NewTreeSitterService()
	ctx := context.Background()

	symbols, err := svc.ExtractSymbols(ctx, []byte{}, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(symbols) != 0 {
		t.Errorf("expected no symbols for empty code, got %d", len(symbols))
	}
}

func TestSymbol_JSON(t *testing.T) {
	symbol := Symbol{
		Name:      "MyFunction",
		Kind:      "function",
		Line:      10,
		Column:    5,
		EndLine:   20,
		EndColumn: 1,
		Signature: "func MyFunction(x int) string",
		Parent:    "MyStruct",
		FilePath:  "main.go",
	}

	data, err := json.Marshal(symbol)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed Symbol
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Name != "MyFunction" {
		t.Errorf("expected name MyFunction, got %s", parsed.Name)
	}

	if parsed.Kind != "function" {
		t.Errorf("expected kind function, got %s", parsed.Kind)
	}

	if parsed.Line != 10 {
		t.Errorf("expected line 10, got %d", parsed.Line)
	}
}

func TestNewServiceWithoutCache(t *testing.T) {
	svc := NewServiceWithoutCache()

	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	if svc.treeSitter == nil {
		t.Error("expected treeSitter to be initialized")
	}

	if svc.cache != nil {
		t.Error("expected cache to be nil")
	}
}

func TestService_ExtractSymbols_NoCache(t *testing.T) {
	svc := NewServiceWithoutCache()
	ctx := context.Background()

	code := []byte(`package main

func Hello() string {
	return "Hello"
}
`)

	symbols, err := svc.ExtractSymbols(ctx, 1, "main.go", "", code, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected symbols to be extracted")
	}

	for _, s := range symbols {
		if s.FilePath != "main.go" {
			t.Errorf("expected FilePath to be main.go, got %s", s.FilePath)
		}
	}
}

func TestService_SearchSymbols_NoCache(t *testing.T) {
	svc := NewServiceWithoutCache()
	ctx := context.Background()

	symbols, err := svc.SearchSymbols(ctx, 1, "test", "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if symbols != nil {
		t.Error("expected nil symbols when no cache")
	}
}

func TestService_InvalidateFile_NoCache(t *testing.T) {
	svc := NewServiceWithoutCache()
	ctx := context.Background()

	err := svc.InvalidateFile(ctx, 1, "main.go")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestService_InvalidateRepo_NoCache(t *testing.T) {
	svc := NewServiceWithoutCache()
	ctx := context.Background()

	err := svc.InvalidateRepo(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
