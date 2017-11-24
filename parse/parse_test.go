package parse

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIsRedundant(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: true,
		},
		{
			name:     "Empty line with newline character",
			input:    "\n",
			expected: true,
		},
		{
			name:     "Comment string (starts with \"--\")",
			input:    "-- This is a comment --",
			expected: true,
		},
		{
			name:     "Set statements (starts with \"SET\")",
			input:    "SET this and that",
			expected: true,
		},
		{
			name:     "OWNER statements",
			input:    "ALTER TABLE set OWNER",
			expected: true,
		},
		{
			name:     "EXTENSION statements",
			input:    "this that bla EXTENSION bla",
			expected: true,
		},
		{
			name:     "Non-trivial line",
			input:    "CREATE TABLE test (",
			expected: false,
		},
	}
	for _, test := range tests {
		if IsRedundant(test.input) != test.expected {
			t.Error(test.name)
		}
	}
}

func TestMapTables(t *testing.T) {
	table1 := []string{
		"CREATE TABLE table1 (",
		"col1 varchar,",
		"col2 string",
		");",
	}
	table2 := []string{
		"CREATE TABLE table2 (",
		");",
	}
	expectedTable1 := &Table{
		Columns: map[string]*Column{
			"col1": &Column{
				Statement: "varchar",
			},
			"col2": &Column{
				Statement: "string",
			},
		},
	}
	expectedTable2 := &Table{}
	expectedTablesMap1 := map[string]*Table{"table1": expectedTable1}
	expectedTablesMap2 := map[string]*Table{"table2": expectedTable2}
	expectedTablesMap3 := map[string]*Table{"table1": expectedTable1, "table2": expectedTable2}

	tests := []struct {
		name           string
		input          []string
		expectedTables map[string]*Table
		expectedLines  []string
	}{
		{
			name:           "No input",
			input:          []string{},
			expectedTables: make(map[string]*Table),
			expectedLines:  []string{},
		},
		{
			name:           "Empty string",
			input:          []string{""},
			expectedTables: make(map[string]*Table),
			expectedLines:  []string{""},
		},
		{
			name:           "Table with columns",
			input:          table1,
			expectedTables: expectedTablesMap1,
			expectedLines:  []string{},
		},
		{
			name:           "Table with no columns",
			input:          table2,
			expectedTables: expectedTablesMap2,
			expectedLines:  []string{},
		},
		{
			name:           "Table statements with extra lines",
			input:          append(append(append([]string{""}, table1...), []string{"", ""}...), table2...),
			expectedTables: expectedTablesMap3,
			expectedLines:  []string{"", "", ""},
		},
	}
	for _, test := range tests {
		tables, lines := MapTables(test.input)
		if !similarTables(tables, test.expectedTables) {
			t.Error(test.name + " - tables error")
		} else if !similarLines(lines, test.expectedLines) {
			t.Error(test.name + " - lines error")
		}
	}
}

func similarTables(tables1, tables2 map[string]*Table) bool {
	if len(tables1) != len(tables2) {
		return false
	} else if len(tables1) == 0 {
		return true
	}
	for name, table1 := range tables1 {
		table2 := tables2[name]
		if table1 == nil && table2 == nil {
			continue
		} else if table1 == nil || table2 == nil || !table1.IsDeepEqual(table2) {
			return false
		}
	}
	return true
}

func similarLines(lines1, lines2 []string) bool {
	if len(lines1) == 0 && len(lines2) == 0 {
		return true
	}
	return cmp.Equal(lines1, lines2)
}
