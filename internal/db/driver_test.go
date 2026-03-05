package db

import (
	"testing"
)

func TestDetectDriver(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected DriverType
	}{
		{
			name:     "PostgreSQL with postgres prefix",
			url:      "postgres://user:pass@localhost:5432/db",
			expected: DriverPostgres,
		},
		{
			name:     "PostgreSQL with postgresql prefix",
			url:      "postgresql://user:pass@localhost:5432/db",
			expected: DriverPostgres,
		},
		{
			name:     "MySQL with mysql prefix",
			url:      "mysql://user:pass@localhost:3306/db",
			expected: DriverMySQL,
		},
		{
			name:     "MySQL DSN format",
			url:      "user:pass@tcp(localhost:3306)/db",
			expected: DriverMySQL,
		},
		{
			name:     "Unknown defaults to postgres",
			url:      "somedb://localhost/db",
			expected: DriverPostgres,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectDriver(tt.url)
			if result != tt.expected {
				t.Errorf("DetectDriver(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestConvertPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		driver   DriverType
		expected string
	}{
		{
			name:     "PostgreSQL keeps placeholders",
			query:    "SELECT * FROM users WHERE id = $1 AND name = $2",
			driver:   DriverPostgres,
			expected: "SELECT * FROM users WHERE id = $1 AND name = $2",
		},
		{
			name:     "MySQL converts placeholders",
			query:    "SELECT * FROM users WHERE id = $1 AND name = $2",
			driver:   DriverMySQL,
			expected: "SELECT * FROM users WHERE id = ? AND name = ?",
		},
		{
			name:     "MySQL with many placeholders",
			query:    "INSERT INTO t (a, b, c, d) VALUES ($1, $2, $3, $4)",
			driver:   DriverMySQL,
			expected: "INSERT INTO t (a, b, c, d) VALUES (?, ?, ?, ?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertPlaceholders(tt.query, tt.driver)
			if result != tt.expected {
				t.Errorf(
					"ConvertPlaceholders(%q, %v) = %q, want %q",
					tt.query,
					tt.driver,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestConvertMySQLURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Full URL with params",
			url:      "mysql://user:pass@localhost:3306/db?charset=utf8",
			expected: "user:pass@tcp(localhost:3306)/db?charset=utf8&parseTime=true",
		},
		{
			name:     "URL without params",
			url:      "mysql://user:pass@localhost:3306/db",
			expected: "user:pass@tcp(localhost:3306)/db?parseTime=true",
		},
		{
			name:     "URL with parseTime already set",
			url:      "mysql://user:pass@localhost:3306/db?parseTime=true",
			expected: "user:pass@tcp(localhost:3306)/db?parseTime=true",
		},
		{
			name:     "DSN format passthrough",
			url:      "user:pass@tcp(localhost:3306)/db",
			expected: "user:pass@tcp(localhost:3306)/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMySQLURL(tt.url)
			if result != tt.expected {
				t.Errorf("convertMySQLURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsNoRowsError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "ErrNoRows",
			err:      ErrNoRows,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNoRowsError(tt.err)
			if result != tt.expected {
				t.Errorf("IsNoRowsError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestSQLBuilderTimestampLiteral(t *testing.T) {
	tests := []struct {
		name     string
		driver   DriverType
		ts       string
		expected string
	}{
		{
			name:     "PostgreSQL timestamp",
			driver:   DriverPostgres,
			ts:       "1970-01-01",
			expected: "'1970-01-01'::timestamptz",
		},
		{
			name:     "PostgreSQL full timestamp",
			driver:   DriverPostgres,
			ts:       "2024-01-15 10:30:00",
			expected: "'2024-01-15 10:30:00'::timestamptz",
		},
		{
			name:     "MySQL timestamp",
			driver:   DriverMySQL,
			ts:       "1970-01-01",
			expected: "'1970-01-01'",
		},
		{
			name:     "MySQL full timestamp",
			driver:   DriverMySQL,
			ts:       "2024-01-15 10:30:00",
			expected: "'2024-01-15 10:30:00'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSQLBuilder(tt.driver)

			result := builder.TimestampLiteral(tt.ts)
			if result != tt.expected {
				t.Errorf("TimestampLiteral(%q) = %q, want %q", tt.ts, result, tt.expected)
			}
		})
	}
}

func TestSQLBuilderCountFilter(t *testing.T) {
	tests := []struct {
		name      string
		driver    DriverType
		condition string
		expected  string
	}{
		{
			name:      "PostgreSQL simple condition",
			driver:    DriverPostgres,
			condition: "status = 'active'",
			expected:  "COUNT(*) FILTER (WHERE status = 'active')",
		},
		{
			name:      "PostgreSQL complex condition",
			driver:    DriverPostgres,
			condition: "last_error IS NOT NULL AND retry_count > 0",
			expected:  "COUNT(*) FILTER (WHERE last_error IS NOT NULL AND retry_count > 0)",
		},
		{
			name:      "MySQL simple condition",
			driver:    DriverMySQL,
			condition: "status = 'active'",
			expected:  "SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END)",
		},
		{
			name:      "MySQL complex condition",
			driver:    DriverMySQL,
			condition: "last_error IS NOT NULL AND retry_count > 0",
			expected:  "SUM(CASE WHEN last_error IS NOT NULL AND retry_count > 0 THEN 1 ELSE 0 END)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSQLBuilder(tt.driver)

			result := builder.CountFilter(tt.condition)
			if result != tt.expected {
				t.Errorf("CountFilter(%q) = %q, want %q", tt.condition, result, tt.expected)
			}
		})
	}
}
