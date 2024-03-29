package parse

import (
	"fmt"
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
			"col1": {Statement: "varchar"},
			"col2": {Statement: "string"},
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

func TestMapSequences(t *testing.T) {
	expectedTable1 := &Table{
		Sequences: []*Sequence{
			{
				Create:   "CREATE SEQUENCE seq;",
				Relation: "ALTER SEQUENCE seq OWNED BY table1.col;",
			},
		},
	}
	expectedTable2 := &Table{
		Sequences: []*Sequence{
			{
				Create:   "CREATE SEQUENCE seq START WITH 2 CACHE 2;",
				Relation: "ALTER SEQUENCE seq OWNED BY table1.col;",
			},
		},
	}
	expectedTablesMap1 := map[string]*Table{"table1": expectedTable1}
	expectedTablesMap2 := map[string]*Table{"table1": expectedTable2}

	tests := []struct {
		name           string
		inputLines     []string
		inputTables    map[string]*Table
		expectedTables map[string]*Table
		expectedLines  []string
		expectedError  error
	}{
		{
			name:           "No input",
			inputLines:     []string{},
			inputTables:    map[string]*Table{"table1": {}},
			expectedTables: map[string]*Table{"table1": {}},
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Table does not exist",
			inputLines:     []string{"CREATE SEQUENCE seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;", "ALTER SEQUENCE seq OWNED BY table1.col;"},
			inputTables:    map[string]*Table{"table2": {}},
			expectedTables: map[string]*Table{"table2": {}},
			expectedLines:  []string{"CREATE SEQUENCE seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;", "ALTER SEQUENCE seq OWNED BY table1.col;"},
			expectedError:  fmt.Errorf("mapping sequences - table does not exist"),
		},
		{
			name:           "Sequence statements with default flags",
			inputLines:     []string{"CREATE SEQUENCE seq START WITH 1 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 1;", "ALTER SEQUENCE seq OWNED BY table1.col;"},
			inputTables:    map[string]*Table{"table1": {}},
			expectedTables: expectedTablesMap1,
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Sequence statements without default flags",
			inputLines:     []string{"CREATE SEQUENCE seq;", "ALTER SEQUENCE seq OWNED BY table1.col;"},
			inputTables:    map[string]*Table{"table1": {}},
			expectedTables: expectedTablesMap1,
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Sequence statements with default flags",
			inputLines:     []string{"CREATE SEQUENCE seq START WITH 2 INCREMENT BY 1 NO MINVALUE NO MAXVALUE CACHE 2;", "ALTER SEQUENCE seq OWNED BY table1.col;"},
			inputTables:    map[string]*Table{"table1": {}},
			expectedTables: expectedTablesMap2,
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Sequence statements with extra lines",
			inputLines:     []string{"\n", "abc", "CREATE SEQUENCE seq;", "ALTER SEQUENCE seq OWNED BY table1.col;", "end"},
			inputTables:    map[string]*Table{"table1": {}},
			expectedTables: expectedTablesMap1,
			expectedLines:  []string{"\n", "abc", "end"},
			expectedError:  nil,
		},
	}
	for _, test := range tests {
		lines, err := MapSequences(test.inputLines, test.inputTables)
		if err != nil && err.Error() != test.expectedError.Error() {
			t.Error(test.name + " - fatal error")
		} else if !similarTables(test.inputTables, test.expectedTables) {
			t.Error(test.name + " - tables error")
		} else if !similarLines(lines, test.expectedLines) {
			t.Error(test.name + " - lines error")
		}
	}
}

func TestMapDefaultValues(t *testing.T) {
	inputTable1 := &Table{
		Columns: map[string]*Column{
			"col1": {Statement: "varchar"},
			"col2": {Statement: "string"},
		},
	}
	inputTable2 := &Table{
		Columns: map[string]*Column{
			"col1": {Statement: "varchar"},
			"col2": {Statement: "string"},
		},
	}
	inputTable3 := &Table{
		Columns: map[string]*Column{
			"col1": {Statement: "varchar"},
			"col2": {Statement: "string"},
		},
	}
	expectedTable1 := &Table{
		Columns: map[string]*Column{
			"col1": {Statement: "varchar DEFAULT nextval('seq'::regclass)"},
			"col2": {Statement: "string"},
		},
	}
	expectedTable2 := &Table{
		Columns: map[string]*Column{
			"col1": {Statement: "varchar"},
			"col2": {Statement: "string DEFAULT nextval('seq'::regclass)"},
		},
	}
	expectedTable3 := &Table{
		Columns: map[string]*Column{
			"col1": {Statement: "varchar DEFAULT nextval('seq'::regclass)"},
			"col2": {Statement: "string"},
		},
	}
	inputTablesMap1 := map[string]*Table{"table1": inputTable1}
	expectedTablesMap1 := map[string]*Table{"table1": expectedTable1}
	inputTablesMap2 := map[string]*Table{"table2": inputTable2}
	expectedTablesMap2 := map[string]*Table{"table2": expectedTable2}
	inputTablesMap3 := map[string]*Table{"table3": inputTable3}
	expectedTablesMap3 := map[string]*Table{"table3": expectedTable3}

	tests := []struct {
		name           string
		inputLines     []string
		inputTables    map[string]*Table
		expectedTables map[string]*Table
		expectedLines  []string
		expectedError  error
	}{
		{
			name:           "No input",
			inputLines:     []string{},
			inputTables:    map[string]*Table{"table1": {}},
			expectedTables: map[string]*Table{"table1": {}},
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "No table",
			inputLines:     []string{"ALTER TABLE ONLY test ALTER COLUMN id SET DEFAULT nextval('seq'::regclass);"},
			inputTables:    map[string]*Table{},
			expectedTables: map[string]*Table{},
			expectedLines:  []string{"ALTER TABLE ONLY test ALTER COLUMN id SET DEFAULT nextval('seq'::regclass);"},
			expectedError:  fmt.Errorf("default value statements found with no mapped tables"),
		},
		{
			name:           "Table does not exist",
			inputLines:     []string{"ALTER TABLE ONLY test ALTER COLUMN id SET DEFAULT nextval('seq'::regclass);"},
			inputTables:    map[string]*Table{"table2": {}},
			expectedTables: map[string]*Table{"table2": {}},
			expectedLines:  []string{"ALTER TABLE ONLY test ALTER COLUMN id SET DEFAULT nextval('seq'::regclass);"},
			expectedError:  fmt.Errorf("mapping default values - table does not exist"),
		},
		{
			name:           "Column does not exist",
			inputLines:     []string{"ALTER TABLE ONLY table1 ALTER COLUMN id SET DEFAULT nextval('seq'::regclass);"},
			inputTables:    map[string]*Table{"table1": {Columns: make(map[string]*Column)}},
			expectedTables: map[string]*Table{"table1": {Columns: make(map[string]*Column)}},
			expectedLines:  []string{"ALTER TABLE ONLY table1 ALTER COLUMN id SET DEFAULT nextval('seq'::regclass);"},
			expectedError:  fmt.Errorf("mapping default values - column does not exist"),
		},
		{
			name:           "Default seq value",
			inputLines:     []string{"ALTER TABLE ONLY table1 ALTER COLUMN col1 SET DEFAULT nextval('seq'::regclass);"},
			inputTables:    inputTablesMap1,
			expectedTables: expectedTablesMap1,
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Alter table does not contain \"ONLY\"",
			inputLines:     []string{"ALTER TABLE table2 ALTER COLUMN col2 SET DEFAULT nextval('seq'::regclass);"},
			inputTables:    inputTablesMap2,
			expectedTables: expectedTablesMap2,
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Set default statements with extra lines",
			inputLines:     []string{"", "abc", "ALTER TABLE ONLY table3 ALTER COLUMN col1 SET DEFAULT nextval('seq'::regclass);", "def"},
			inputTables:    inputTablesMap3,
			expectedTables: expectedTablesMap3,
			expectedLines:  []string{"", "abc", "def"},
			expectedError:  nil,
		},
	}
	for _, test := range tests {
		lines, err := MapDefaultValues(test.inputLines, test.inputTables)
		if err != nil && err.Error() != test.expectedError.Error() {
			t.Error(test.name + " - fatal error")
		} else if !similarTables(test.inputTables, test.expectedTables) {
			t.Error(test.name + " - tables error")
		} else if !similarLines(lines, test.expectedLines) {
			t.Error(test.name + " - lines error")
		}
	}
}

func TestMapConstraints(t *testing.T) {
	inputTable1 := &Table{
		Columns: map[string]*Column{
			"id": {Statement: "col id"},
		},
		Constraints: make(map[string]string),
	}
	inputTable2 := &Table{
		Columns: map[string]*Column{
			"id": {Statement: "col id"},
		},
		Constraints: make(map[string]string),
	}
	inputTable3 := &Table{
		Columns: map[string]*Column{
			"id": {Statement: "col id"},
		},
		Constraints: make(map[string]string),
	}
	expectedTable1 := &Table{
		Columns: map[string]*Column{
			"id": {
				Statement:    "col id",
				IsPrimaryKey: true,
			},
		},
		Constraints: map[string]string{
			"table_pkey": "CONSTRAINT table_pkey PRIMARY KEY (id)",
		},
	}
	expectedTable2 := &Table{
		Columns: map[string]*Column{
			"id": {
				Statement:    "col id",
				IsForeignKey: true,
			},
		},
		Constraints: map[string]string{
			"table_fkey": "CONSTRAINT table_fkey FOREIGN KEY (id) REFERENCES table2(id) ON DELETE CASCADE",
		},
	}
	expectedTable3 := &Table{
		Columns: map[string]*Column{
			"id": {
				Statement:    "col id",
				IsPrimaryKey: true,
			},
		},
		Constraints: map[string]string{
			"table_pkey": "CONSTRAINT table_pkey PRIMARY KEY (id)",
		},
	}
	inputTablesMap1 := map[string]*Table{"table1": inputTable1}
	expectedTablesMap1 := map[string]*Table{"table1": expectedTable1}
	inputTablesMap2 := map[string]*Table{"table2": inputTable2}
	expectedTablesMap2 := map[string]*Table{"table2": expectedTable2}
	inputTablesMap3 := map[string]*Table{"table3": inputTable3}
	expectedTablesMap3 := map[string]*Table{"table3": expectedTable3}

	tests := []struct {
		name           string
		inputLines     []string
		inputTables    map[string]*Table
		expectedTables map[string]*Table
		expectedLines  []string
		expectedError  error
	}{
		{
			name:           "No input",
			inputLines:     []string{},
			inputTables:    map[string]*Table{"table1": {}},
			expectedTables: map[string]*Table{"table1": {}},
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "No table",
			inputLines:     []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey PRIMARY KEY (id);"},
			inputTables:    map[string]*Table{},
			expectedTables: map[string]*Table{},
			expectedLines:  []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey PRIMARY KEY (id);"},
			expectedError:  fmt.Errorf("constraint statements found with no mapped tables"),
		},
		{
			name:           "Table does not exist",
			inputLines:     []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey PRIMARY KEY (id);"},
			inputTables:    map[string]*Table{"table2": {}},
			expectedTables: map[string]*Table{"table2": {}},
			expectedLines:  []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey PRIMARY KEY (id);"},
			expectedError:  fmt.Errorf("mapping constraints - table does not exist"),
		},
		{
			name:           "Column does not exist - primary key",
			inputLines:     []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey PRIMARY KEY (id);"},
			inputTables:    map[string]*Table{"table1": {Constraints: make(map[string]string)}},
			expectedTables: map[string]*Table{"table1": {Constraints: make(map[string]string)}},
			expectedLines:  []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey PRIMARY KEY (id);"},
			expectedError:  fmt.Errorf("mapping constraints - column does not exist"),
		},
		{
			name:           "Column does not exist - foreign key",
			inputLines:     []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey FOREIGN KEY (id);"},
			inputTables:    map[string]*Table{"table1": {Constraints: make(map[string]string)}},
			expectedTables: map[string]*Table{"table1": {Constraints: make(map[string]string)}},
			expectedLines:  []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey FOREIGN KEY (id);"},
			expectedError:  fmt.Errorf("column does not exist"),
		},
		{
			name:           "Primary key constraint",
			inputLines:     []string{"ALTER TABLE ONLY table1 ADD CONSTRAINT table_pkey PRIMARY KEY (id);"},
			inputTables:    inputTablesMap1,
			expectedTables: expectedTablesMap1,
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Foreign key constraint",
			inputLines:     []string{"ALTER TABLE ONLY table2 ADD CONSTRAINT table_fkey FOREIGN KEY (id) REFERENCES table2(id) ON DELETE CASCADE;"},
			inputTables:    inputTablesMap2,
			expectedTables: expectedTablesMap2,
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Constraint statements with extra lines",
			inputLines:     []string{"", "abc", "ALTER TABLE ONLY table3 ADD CONSTRAINT table_pkey PRIMARY KEY (id);", "def"},
			inputTables:    inputTablesMap3,
			expectedTables: expectedTablesMap3,
			expectedLines:  []string{"", "abc", "def"},
			expectedError:  nil,
		},
	}
	for _, test := range tests {
		lines, err := MapConstraints(test.inputLines, test.inputTables)
		if err != nil && err.Error() != test.expectedError.Error() {
			t.Error(test.name + " - fatal error")
		} else if !similarTables(test.inputTables, test.expectedTables) {
			fmt.Println(test.inputTables["table1"])
			fmt.Println(test.expectedTables["table1"])
			t.Error(test.name + " - tables error")
		} else if !similarLines(lines, test.expectedLines) {
			t.Error(test.name + " - lines error")
		}
	}
}

func TestMapIndices(t *testing.T) {
	inputTable1 := &Table{}
	inputTable2 := &Table{}
	expectedTable1 := &Table{Index: []string{"CREATE UNIQUE INDEX user_idx ON table1 USING btree (username);"}}
	expectedTable2 := &Table{Index: []string{"CREATE UNIQUE INDEX user_idx ON table2 USING btree (username);"}}
	inputTablesMap1 := map[string]*Table{"table1": inputTable1}
	expectedTablesMap1 := map[string]*Table{"table1": expectedTable1}
	inputTablesMap2 := map[string]*Table{"table2": inputTable2}
	expectedTablesMap2 := map[string]*Table{"table2": expectedTable2}

	tests := []struct {
		name           string
		inputLines     []string
		inputTables    map[string]*Table
		expectedTables map[string]*Table
		expectedLines  []string
		expectedError  error
	}{
		{
			name:           "No input",
			inputLines:     []string{},
			inputTables:    map[string]*Table{"table1": {}},
			expectedTables: map[string]*Table{"table1": {}},
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "No table",
			inputLines:     []string{"CREATE UNIQUE INDEX user_idx ON table1 USING btree (username);"},
			inputTables:    map[string]*Table{},
			expectedTables: map[string]*Table{},
			expectedLines:  []string{"CREATE UNIQUE INDEX user_idx ON table1 USING btree (username);"},
			expectedError:  fmt.Errorf("index statements found with no mapped tables"),
		},
		{
			name:           "Table does not exist",
			inputLines:     []string{"CREATE UNIQUE INDEX user_idx ON table1 USING btree (username);"},
			inputTables:    map[string]*Table{"table2": {}},
			expectedTables: map[string]*Table{"table2": {}},
			expectedLines:  []string{"CREATE UNIQUE INDEX user_idx ON table1 USING btree (username);"},
			expectedError:  fmt.Errorf("mapping indices - table does not exist"),
		},
		{
			name:           "Create index",
			inputLines:     []string{"CREATE UNIQUE INDEX user_idx ON table1 USING btree (username);"},
			inputTables:    inputTablesMap1,
			expectedTables: expectedTablesMap1,
			expectedLines:  []string{},
			expectedError:  nil,
		},
		{
			name:           "Index statements with extra lines",
			inputLines:     []string{"", "abc", "CREATE UNIQUE INDEX user_idx ON table2 USING btree (username);", "def"},
			inputTables:    inputTablesMap2,
			expectedTables: expectedTablesMap2,
			expectedLines:  []string{"", "abc", "def"},
			expectedError:  nil,
		},
	}
	for _, test := range tests {
		lines, err := MapIndices(test.inputLines, test.inputTables)
		if err != nil && err.Error() != test.expectedError.Error() {
			t.Error(test.name + " - fatal error")
		} else if !similarTables(test.inputTables, test.expectedTables) {
			t.Error(test.name + " - tables error")
		} else if !similarLines(lines, test.expectedLines) {
			t.Error(test.name + " - lines error")
		}
	}
}

func TestSquashMultiLineStatements(t *testing.T) {
	input1 := []string{"abc", "def", "ghi;"}
	expected1 := []string{"abc def ghi;"}
	input2 := []string{"1234", "5;", "abc", "def;", "ghi;"}
	expected2 := []string{"1234 5;", "abc def;", "ghi;"}

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "No input",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "Lines",
			input:    input1,
			expected: expected1,
		},
		{
			name:     "Multiple lines",
			input:    input2,
			expected: expected2,
		},
	}
	for _, test := range tests {
		lines := SquashMultiLineStatements(test.input)
		if !similarLines(lines, test.expected) {
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
