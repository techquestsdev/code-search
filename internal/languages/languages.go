// Package languages provides shared language detection markers used by both
// the indexer (bare-repo language detection) and the SCIP service (project
// directory discovery). Keeping the canonical list in one place avoids drift
// between the two subsystems.
package languages

// Language describes a programming language and the files whose presence at a
// project root indicates the language is in use.
type Language struct {
	Name    string
	Markers []string
}

// All is the ordered list of supported languages. The order defines detection
// priority — more specific languages come first.
var All = []Language{
	{Name: "go", Markers: []string{"go.mod"}},
	{Name: "typescript", Markers: []string{"tsconfig.json"}},
	{Name: "javascript", Markers: []string{"package.json"}},
	{Name: "rust", Markers: []string{"Cargo.toml"}},
	{Name: "java", Markers: []string{"pom.xml", "build.gradle", "build.gradle.kts"}},
	{Name: "python", Markers: []string{"pyproject.toml", "setup.py", "requirements.txt"}},
	{Name: "php", Markers: []string{"composer.json"}},
}

// Priority returns language names in detection order.
func Priority() []string {
	names := make([]string, len(All))
	for i, l := range All {
		names[i] = l.Name
	}
	return names
}

// MarkersByLanguage returns a map from language name to its marker files.
func MarkersByLanguage() map[string][]string {
	m := make(map[string][]string, len(All))
	for _, l := range All {
		m[l.Name] = l.Markers
	}
	return m
}
