package scip

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	scipproto "github.com/sourcegraph/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

// Parser handles parsing and importing SCIP index files.
type Parser struct {
	store *Store
}

// NewParser creates a new SCIP parser.
func NewParser(store *Store) *Parser {
	return &Parser{store: store}
}

// ImportFromFile imports a SCIP index from a file path.
func (p *Parser) ImportFromFile(ctx context.Context, repoID int64, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open SCIP file: %w", err)
	}
	defer f.Close()

	return p.ImportFromReader(ctx, repoID, f)
}

// ImportFromReader imports a SCIP index from a reader.
func (p *Parser) ImportFromReader(ctx context.Context, repoID int64, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read SCIP data: %w", err)
	}

	return p.ImportFromBytes(ctx, repoID, data)
}

// ImportFromBytes imports a SCIP index from raw bytes.
func (p *Parser) ImportFromBytes(ctx context.Context, repoID int64, data []byte) error {
	return p.ImportFromBytesWithPrefix(ctx, repoID, data, "")
}

// ImportFromBytesWithPrefix imports a SCIP index from raw bytes, prefixing all file paths.
// It clears any existing index before importing.
func (p *Parser) ImportFromBytesWithPrefix(
	ctx context.Context,
	repoID int64,
	data []byte,
	pathPrefix string,
) error {
	var index scipproto.Index

	err := proto.Unmarshal(data, &index)
	if err != nil {
		return fmt.Errorf("failed to unmarshal SCIP index: %w", err)
	}

	return p.ImportIndexWithPrefix(ctx, repoID, &index, pathPrefix)
}

// AppendFromBytesWithPrefix imports a SCIP index from raw bytes, prefixing all file paths.
// Unlike ImportFromBytesWithPrefix, it does NOT clear the existing index first,
// allowing multiple project indexes to be merged into a single repo index (for monorepos).
func (p *Parser) AppendFromBytesWithPrefix(
	ctx context.Context,
	repoID int64,
	data []byte,
	pathPrefix string,
) error {
	var index scipproto.Index

	err := proto.Unmarshal(data, &index)
	if err != nil {
		return fmt.Errorf("failed to unmarshal SCIP index: %w", err)
	}

	return p.appendIndexWithPrefix(ctx, repoID, &index, pathPrefix)
}

// ImportIndex imports a parsed SCIP index into the store.
func (p *Parser) ImportIndex(ctx context.Context, repoID int64, index *scipproto.Index) error {
	return p.ImportIndexWithPrefix(ctx, repoID, index, "")
}

// ImportIndexWithPrefix imports a parsed SCIP index into the store, prefixing all file paths.
// It clears any existing index before importing.
func (p *Parser) ImportIndexWithPrefix(
	ctx context.Context,
	repoID int64,
	index *scipproto.Index,
	pathPrefix string,
) error {
	// Clear existing index
	if err := p.store.ClearIndex(ctx, repoID); err != nil {
		return fmt.Errorf("failed to clear existing index: %w", err)
	}

	return p.appendIndexWithPrefix(ctx, repoID, index, pathPrefix)
}

// appendIndexWithPrefix imports a parsed SCIP index into the store, prefixing all file paths.
// It does NOT clear the existing index, allowing multiple imports to be merged.
func (p *Parser) appendIndexWithPrefix(
	ctx context.Context,
	repoID int64,
	index *scipproto.Index,
	pathPrefix string,
) error {
	db, err := p.store.getDB(repoID)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Store metadata
	if index.GetMetadata() != nil {
		err := p.storeMetadata(ctx, tx, index.GetMetadata())
		if err != nil {
			return err
		}
	}

	// Prepare statements
	occStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO occurrences (symbol, file_path, start_line, start_col, end_line, end_col, role, syntax_kind)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer occStmt.Close()

	infoStmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO symbol_info (symbol, documentation, kind, display_name, enclosing_symbol)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer infoStmt.Close()

	relStmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO symbol_relationships (symbol, related_symbol, relationship_type)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer relStmt.Close()

	// Process documents
	for _, doc := range index.GetDocuments() {
		// Skip invalid paths (e.g., Go build cache files with relative paths outside repo)
		if !isValidSourcePath(doc.GetRelativePath()) {
			continue
		}

		// Apply path prefix if provided (e.g., when project is in a subdirectory)
		originalPath := doc.GetRelativePath()
		if pathPrefix != "" {
			doc.RelativePath = filepath.Join(pathPrefix, doc.GetRelativePath())
		}

		err := p.processDocument(ctx, occStmt, infoStmt, relStmt, doc)
		if err != nil {
			return fmt.Errorf("failed to process document %s: %w", originalPath, err)
		}
	}

	// Process external symbols
	for _, sym := range index.GetExternalSymbols() {
		err := p.processSymbolInfo(ctx, infoStmt, relStmt, sym)
		if err != nil {
			return fmt.Errorf("failed to process external symbol %s: %w", sym.GetSymbol(), err)
		}
	}

	return tx.Commit()
}

func (p *Parser) storeMetadata(ctx context.Context, tx *sql.Tx, meta *scipproto.Metadata) error {
	items := map[string]string{
		"version":      fmt.Sprintf("%d", meta.GetVersion()),
		"project_root": meta.GetProjectRoot(),
		"indexed_at":   time.Now().UTC().Format(time.RFC3339),
	}

	if meta.GetToolInfo() != nil {
		items["tool_name"] = meta.GetToolInfo().GetName()
		items["tool_version"] = meta.GetToolInfo().GetVersion()
	}

	if meta.GetTextDocumentEncoding() != 0 {
		items["text_encoding"] = meta.GetTextDocumentEncoding().String()
	}

	stmt, err := tx.PrepareContext(
		ctx,
		"INSERT OR REPLACE INTO index_meta (key, value) VALUES (?, ?)",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for k, v := range items {
		if _, err := stmt.ExecContext(ctx, k, v); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) processDocument(
	ctx context.Context,
	occStmt, infoStmt, relStmt *sql.Stmt,
	doc *scipproto.Document,
) error {
	filePath := doc.GetRelativePath()

	// Process occurrences
	for _, occ := range doc.GetOccurrences() {
		startLine, startCol, endLine, endCol := decodeRange(occ.GetRange())

		symbol := occ.GetSymbol()
		if symbol == "" {
			continue // Skip occurrences without symbols
		}

		role := int(occ.GetSymbolRoles())
		syntaxKind := int(occ.GetSyntaxKind())

		if _, err := occStmt.ExecContext(ctx, symbol, filePath, startLine, startCol, endLine, endCol, role, syntaxKind); err != nil {
			return err
		}
	}

	// Process symbol information from document
	for _, sym := range doc.GetSymbols() {
		err := p.processSymbolInfo(ctx, infoStmt, relStmt, sym)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) processSymbolInfo(
	ctx context.Context,
	infoStmt, relStmt *sql.Stmt,
	sym *scipproto.SymbolInformation,
) error {
	if sym.GetSymbol() == "" {
		return nil
	}

	// Extract documentation
	doc := ""
	if len(sym.GetDocumentation()) > 0 {
		doc = strings.Join(sym.GetDocumentation(), "\n")
	}

	// Store symbol info
	var kind sql.NullInt64
	if sym.GetKind() != 0 {
		kind = sql.NullInt64{Int64: int64(sym.GetKind()), Valid: true}
	}

	var displayName sql.NullString
	if sym.GetDisplayName() != "" {
		displayName = sql.NullString{String: sym.GetDisplayName(), Valid: true}
	}

	var enclosing sql.NullString
	if sym.GetEnclosingSymbol() != "" {
		enclosing = sql.NullString{String: sym.GetEnclosingSymbol(), Valid: true}
	}

	if _, err := infoStmt.ExecContext(ctx, sym.GetSymbol(), doc, kind, displayName, enclosing); err != nil {
		return err
	}

	// Store relationships
	for _, rel := range sym.GetRelationships() {
		relType := "references"
		if rel.GetIsImplementation() {
			relType = "implements"
		} else if rel.GetIsTypeDefinition() {
			relType = "type_definition"
		} else if rel.GetIsReference() {
			relType = "references"
		}

		if _, err := relStmt.ExecContext(ctx, sym.GetSymbol(), rel.GetSymbol(), relType); err != nil {
			return err
		}
	}

	return nil
}

// decodeRange decodes a SCIP range into start/end line/col.
// SCIP ranges are encoded as [startLine, startChar, endLine?, endChar]
// If 3 elements: same line, so endLine = startLine.
func decodeRange(r []int32) (startLine, startCol, endLine, endCol int) {
	if len(r) < 3 {
		return 0, 0, 0, 0
	}

	startLine = int(r[0])
	startCol = int(r[1])

	if len(r) == 3 {
		// Same line
		endLine = startLine
		endCol = int(r[2])
	} else if len(r) >= 4 {
		endLine = int(r[2])
		endCol = int(r[3])
	}

	return
}

// ParseSymbol parses a SCIP symbol string into its components.
// Symbol format: "<scheme> <package> <descriptors>..." or "local <id>".
func ParseSymbol(symbol string) (scheme, pkg string, descriptors []string, isLocal bool) {
	if strings.HasPrefix(symbol, "local ") {
		return "", "", []string{symbol[6:]}, true
	}

	parts := strings.SplitN(symbol, " ", 3)
	if len(parts) < 2 {
		return "", "", nil, false
	}

	scheme = parts[0]
	pkg = parts[1]

	if len(parts) > 2 {
		// Parse descriptors (simplified)
		descriptors = strings.Split(parts[2], "/")
	}

	return
}

// SymbolToHumanReadable converts a SCIP symbol to a human-readable name.
func SymbolToHumanReadable(symbol string) string {
	if strings.HasPrefix(symbol, "local ") {
		return symbol[6:]
	}

	parts := strings.SplitN(symbol, " ", 3)
	if len(parts) < 3 {
		return symbol
	}

	// Extract the last descriptor which is usually the symbol name
	descriptors := parts[2]

	// Remove method/type markers like () # . etc
	name := descriptors
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// Clean up common suffixes
	name = strings.TrimSuffix(name, ".")
	name = strings.TrimSuffix(name, "#")
	name = strings.TrimSuffix(name, "()")

	return name
}

// isValidSourcePath checks if a file path is a valid source file path.
// It filters out Go build cache files and other invalid paths.
func isValidSourcePath(path string) bool {
	// Skip empty paths
	if path == "" {
		return false
	}

	// Skip paths that escape the repository (start with ..)
	if strings.HasPrefix(path, "..") {
		return false
	}

	// Skip absolute paths
	if filepath.IsAbs(path) {
		return false
	}

	// Skip Go build cache paths
	if strings.Contains(path, "go-build") || strings.Contains(path, "go/pkg/mod") {
		return false
	}

	// Skip vendor directories (optional, may want to include these)
	// if strings.HasPrefix(path, "vendor/") {
	// 	return false
	// }

	return true
}
