package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// StringArray is a database-agnostic string array type.
// For PostgreSQL, it uses native TEXT[] type.
// For MySQL, it serializes to JSON.
type StringArray []string

// Value implements the driver.Valuer interface.
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}

	if len(a) == 0 {
		return "{}", nil
	}
	// Return PostgreSQL array literal format: {"val1","val2","val3"}
	// This works for PostgreSQL TEXT[] columns
	// For MySQL, the Scan method handles JSON format on read
	var escaped []string

	for _, s := range a {
		// Escape quotes and backslashes for PostgreSQL array format
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		escaped = append(escaped, `"`+s+`"`)
	}

	return "{" + strings.Join(escaped, ",") + "}", nil
}

// Scan implements the sql.Scanner interface.
func (a *StringArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		// Check for binary PostgreSQL array format (starts with dimension info)
		// Binary arrays start with: number of dimensions (4 bytes), flags (4 bytes), element OID (4 bytes)
		if len(v) >= 12 && v[0] == 0 && v[1] == 0 && v[2] == 0 {
			// This is binary format - parse it
			return a.scanBinaryPostgresArray(v)
		}
		// Check for PostgreSQL array format: {val1,val2,val3}
		if len(v) > 0 && v[0] == '{' {
			return a.scanPostgresArray(string(v))
		}
		// Try JSON array format: ["val1","val2"]
		if len(v) > 0 && v[0] == '[' {
			return json.Unmarshal(v, a)
		}
		// Empty or other - try as PostgreSQL array
		return a.scanPostgresArray(string(v))
	case string:
		// Check for PostgreSQL array format: {val1,val2,val3}
		if len(v) > 0 && v[0] == '{' {
			return a.scanPostgresArray(v)
		}
		// Try JSON array format: ["val1","val2"]
		if len(v) > 0 && v[0] == '[' {
			return json.Unmarshal([]byte(v), a)
		}
		// Empty or other - try as PostgreSQL array
		return a.scanPostgresArray(v)
	case []string:
		*a = v
		return nil
	case pgtype.FlatArray[string]:
		*a = []string(v)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into StringArray", src)
	}
}

// scanBinaryPostgresArray parses binary format PostgreSQL arrays.
// Binary format: [dims:4][flags:4][elemOID:4][dim1_len:4][dim1_lbound:4]...[elements...].
func (a *StringArray) scanBinaryPostgresArray(data []byte) error {
	if len(data) < 12 {
		*a = []string{}
		return nil
	}

	// Read number of dimensions (first 4 bytes, big-endian)
	ndim := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])

	if ndim == 0 {
		// Empty array
		*a = []string{}
		return nil
	}

	if ndim != 1 {
		// We only support 1-dimensional arrays
		*a = []string{}
		return nil
	}

	// Skip flags (4 bytes) and element OID (4 bytes)
	// Read array length at offset 12
	if len(data) < 20 {
		*a = []string{}
		return nil
	}

	arrayLen := int(data[12])<<24 | int(data[13])<<16 | int(data[14])<<8 | int(data[15])

	if arrayLen == 0 {
		*a = []string{}
		return nil
	}

	// Parse elements starting at offset 20
	result := make([]string, 0, arrayLen)
	offset := 20

	for i := 0; i < arrayLen && offset < len(data); i++ {
		if offset+4 > len(data) {
			break
		}

		// Read element length (4 bytes, -1 means NULL)
		elemLen := int(
			int32(
				data[offset],
			)<<24 | int32(
				data[offset+1],
			)<<16 | int32(
				data[offset+2],
			)<<8 | int32(
				data[offset+3],
			),
		)
		offset += 4

		if elemLen == -1 {
			// NULL element, skip
			continue
		}

		if offset+elemLen > len(data) {
			break
		}

		result = append(result, string(data[offset:offset+elemLen]))
		offset += elemLen
	}

	*a = result

	return nil
}

// scanPostgresArray parses PostgreSQL array format {val1,val2,val3}.
func (a *StringArray) scanPostgresArray(s string) error {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")

	if s == "" {
		*a = []string{}
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]string, len(parts))

	for i, p := range parts {
		// Remove quotes if present
		p = strings.Trim(p, "\"")
		result[i] = p
	}

	*a = result

	return nil
}

// SQLBuilder helps build database-agnostic SQL queries.
type SQLBuilder struct {
	driver DriverType
}

// NewSQLBuilder creates a new SQL builder for the given driver.
func NewSQLBuilder(driver DriverType) *SQLBuilder {
	return &SQLBuilder{driver: driver}
}

// ILike returns a case-insensitive LIKE condition.
// PostgreSQL: column ILIKE pattern
// MySQL: LOWER(column) LIKE LOWER(pattern).
func (b *SQLBuilder) ILike(column string) string {
	if b.driver == DriverMySQL {
		return fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", column)
	}

	return column + " ILIKE $1"
}

// ILikeArg returns the placeholder for ILIKE condition at given position.
func (b *SQLBuilder) ILikeWithPos(column string, pos int) string {
	if b.driver == DriverMySQL {
		return fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", column)
	}

	return fmt.Sprintf("%s ILIKE $%d", column, pos)
}

// Concat returns a string concatenation expression.
// PostgreSQL: a || b || c
// MySQL: CONCAT(a, b, c).
func (b *SQLBuilder) Concat(parts ...string) string {
	if b.driver == DriverMySQL {
		return "CONCAT(" + strings.Join(parts, ", ") + ")"
	}

	return strings.Join(parts, " || ")
}

// Now returns the current timestamp function.
func (b *SQLBuilder) Now() string {
	// Both support NOW() and CURRENT_TIMESTAMP
	return "NOW()"
}

// Placeholder returns the placeholder for the given position.
func (b *SQLBuilder) Placeholder(pos int) string {
	if b.driver == DriverMySQL {
		return "?"
	}

	return fmt.Sprintf("$%d", pos)
}

// ReturningID returns a RETURNING clause or empty string for MySQL.
// For MySQL, you need to use LastInsertId() instead.
func (b *SQLBuilder) ReturningID() string {
	if b.driver == DriverMySQL {
		return ""
	}

	return "RETURNING id"
}

// Returning returns a RETURNING clause or empty string for MySQL.
func (b *SQLBuilder) Returning(columns ...string) string {
	if b.driver == DriverMySQL {
		return ""
	}

	return "RETURNING " + strings.Join(columns, ", ")
}

// UpsertQuery builds an upsert query.
// PostgreSQL: INSERT ... ON CONFLICT (key) DO UPDATE SET ...
// MySQL: INSERT ... ON DUPLICATE KEY UPDATE ...
func (b *SQLBuilder) UpsertQuery(
	table string,
	columns []string,
	conflictKey string,
	updateColumns []string,
) string {
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = b.Placeholder(i + 1)
	}

	insertPart := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	if b.driver == DriverMySQL {
		updates := make([]string, len(updateColumns))
		for i, col := range updateColumns {
			updates[i] = fmt.Sprintf("%s = VALUES(%s)", col, col)
		}

		return insertPart + " ON DUPLICATE KEY UPDATE " + strings.Join(updates, ", ")
	}

	// PostgreSQL
	updates := make([]string, len(updateColumns))
	for i, col := range updateColumns {
		updates[i] = fmt.Sprintf("%s = EXCLUDED.%s", col, col)
	}

	return fmt.Sprintf("%s ON CONFLICT (%s) DO UPDATE SET %s",
		insertPart, conflictKey, strings.Join(updates, ", "))
}

// ArrayContains checks if an array contains a value.
// PostgreSQL: $1 = ANY(column)
// MySQL: JSON_CONTAINS(column, JSON_QUOTE($1)).
func (b *SQLBuilder) ArrayContains(column string, pos int) string {
	if b.driver == DriverMySQL {
		return fmt.Sprintf("JSON_CONTAINS(%s, JSON_QUOTE(?))", column)
	}

	return fmt.Sprintf("$%d = ANY(%s)", pos, column)
}

// ArrayLength returns the length of an array.
// PostgreSQL: cardinality(column) or array_length(column, 1)
// MySQL: JSON_LENGTH(column).
func (b *SQLBuilder) ArrayLength(column string) string {
	if b.driver == DriverMySQL {
		return fmt.Sprintf("JSON_LENGTH(%s)", column)
	}

	return fmt.Sprintf("cardinality(%s)", column)
}

// LikePattern builds a LIKE pattern with wildcards.
func (b *SQLBuilder) LikePattern(value string) string {
	return "%" + value + "%"
}

// ForUpdateSkipLocked returns the appropriate locking clause.
// Both PostgreSQL and MySQL 8.0+ support this syntax.
func (b *SQLBuilder) ForUpdateSkipLocked() string {
	return "FOR UPDATE SKIP LOCKED"
}

// TimestampLiteral converts a timestamp string to database-specific format.
// PostgreSQL: '1970-01-01'::timestamptz
// MySQL: '1970-01-01' (implicit conversion).
func (b *SQLBuilder) TimestampLiteral(ts string) string {
	if b.driver == DriverMySQL {
		return fmt.Sprintf("'%s'", ts)
	}

	return fmt.Sprintf("'%s'::timestamptz", ts)
}

// CountFilter returns a conditional count expression.
// PostgreSQL: COUNT(*) FILTER (WHERE condition)
// MySQL: SUM(CASE WHEN condition THEN 1 ELSE 0 END).
func (b *SQLBuilder) CountFilter(condition string) string {
	if b.driver == DriverMySQL {
		return fmt.Sprintf("SUM(CASE WHEN %s THEN 1 ELSE 0 END)", condition)
	}

	return fmt.Sprintf("COUNT(*) FILTER (WHERE %s)", condition)
}
