package parse

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/jchiam/psql-schema-dump-sanitiser/graph"
)

// Column is the struct containing logical aspects of a table column
type Column struct {
	Statement    string
	IsPrimaryKey bool
	IsForeignKey bool
}

// Sequence holds the create and relation statements of a table
type Sequence struct {
	Create   string
	Relation string
}

// Table is the struct containing logical aspects of a psql table's structure
type Table struct {
	Columns     map[string]*Column
	Constraints map[string]string
	Sequences   []*Sequence
	Index       []string
}

func similarColumns(cols1, cols2 map[string]*Column) bool {
	if len(cols1) != len(cols2) {
		return false
	} else if len(cols1) == 0 {
		return true
	}
	for col := range cols1 {
		if !cmp.Equal(cols1[col], cols2[col]) {
			return false
		}
	}
	return true
}

func similarSequences(seqs1, seqs2 []*Sequence) bool {
	if len(seqs1) != len(seqs2) {
		return false
	} else if len(seqs1) == 0 {
		return true
	}
	for i, seq1 := range seqs1 {
		if !cmp.Equal(seq1, seqs2[i]) {
			return false
		}
	}
	return true
}

func similarConstraints(cons1, cons2 map[string]string) bool {
	if len(cons1) != len(cons2) {
		return false
	} else if len(cons1) == 0 {
		return true
	}
	for cons := range cons1 {
		if !cmp.Equal(cons1[cons], cons2[cons]) {
			return false
		}
	}
	return true
}

// IsDeepEqual compares the two tables and returns whether they are deeply equal
func (t Table) IsDeepEqual(table *Table) bool {
	if !similarColumns(t.Columns, table.Columns) || !similarSequences(t.Sequences, table.Sequences) ||
		!similarConstraints(t.Constraints, table.Constraints) || !cmp.Equal(t.Sequences, table.Sequences) ||
		!cmp.Equal(t.Index, table.Index) {
		return false
	}
	return true
}

// IsRedundant checks if line is a redundant sql statement or comment
func IsRedundant(line string) bool {
	if len(line) == 0 || line == "\n" {
		return true
	}

	tokens := strings.Split(line, " ")
	// Skip lines that start with "--", "SET"
	if tokens[0] == "--" || tokens[0] == "SET" {
		return true
	}
	// Skip extension and owner statements
	if strings.Contains(line, "EXTENSION") || strings.Contains(line, "OWNER") {
		return true
	}
	// Skip config select statements
	if strings.Contains(line, "SELECT pg_catalog.set_config") {
		return true
	}

	return false
}

func removeAccessModifier(s string) (string, string) {
	tokens := strings.Split(s, ".")
	if len(tokens) > 1 {
		return tokens[1], tokens[0]
	}
	return tokens[0], ""
}

// MapTables parses sql statements and returns a map of Table structs containing information of table's structure
// and the remaining unprocessed lines
func MapTables(lines []string) (map[string]*Table, []string) {
	tables := make(map[string]*Table)
	if len(lines) == 0 {
		return tables, lines
	}

	var bufferLines []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "CREATE TABLE") {
			tableName, _ := removeAccessModifier(strings.Split(line, " ")[2])
			table := Table{
				Columns:     make(map[string]*Column),
				Constraints: make(map[string]string),
			}

			j := i + 1
			for ; lines[j][len(lines[j])-1] != ';'; j++ {
				columnLine := strings.Trim(lines[j], " ")
				spaceIndex := strings.Index(columnLine, " ")
				columnName := columnLine[:spaceIndex]
				if columnLine[len(columnLine)-1] == ',' {
					table.Columns[columnName] = &Column{
						Statement:    columnLine[spaceIndex+1 : len(columnLine)-1],
						IsPrimaryKey: false,
						IsForeignKey: false,
					}
				} else {
					table.Columns[columnName] = &Column{
						Statement:    columnLine[spaceIndex+1:],
						IsPrimaryKey: false,
						IsForeignKey: false,
					}
				}
			}

			tables[tableName] = &table
			i = j
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return tables, bufferLines
}

func simplifyCreateSequenceStatement(stmt string) string {
	if len(stmt) == 0 || !strings.Contains(stmt, "CREATE SEQUENCE") {
		return stmt
	}

	index := strings.Index(stmt, "START WITH 1")
	if index != -1 {
		stmt = stmt[:index] + stmt[index+len("START WITH 1"):]
	}
	index = strings.Index(stmt, "INCREMENT BY 1")
	if index != -1 {
		stmt = stmt[:index] + stmt[index+len("INCREMENT BY 1"):]
	}
	index = strings.Index(stmt, "NO MINVALUE")
	if index != -1 {
		stmt = stmt[:index] + stmt[index+len("NO MINVALUE"):]
	}
	index = strings.Index(stmt, "NO MAXVALUE")
	if index != -1 {
		stmt = stmt[:index] + stmt[index+len("NO MAXVALUE"):]
	}
	index = strings.Index(stmt, "CACHE 1")
	if index != -1 {
		stmt = stmt[:index] + stmt[index+len("CACHE 1"):]
	}

	multipleWhiteSpaceExp := regexp.MustCompile(`[\s]{2,}`)
	stmt = multipleWhiteSpaceExp.ReplaceAllString(stmt, " ")

	spacesBeforeSemicolon := regexp.MustCompile(`[\s]{1,};`)
	stmt = spacesBeforeSemicolon.ReplaceAllString(stmt, ";")

	return stmt
}

// MapSequences parses sql statements and squashes them into a single create sequence statement amd mapped to tables.
// It then returns the remaining lines.
// Note: Assumes relation statement is always after create statement.
func MapSequences(lines []string, tables map[string]*Table) ([]string, error) {
	if len(lines) == 0 {
		return lines, nil
	}

	var bufferLines []string
	for i, line := range lines {
		if strings.Contains(line, "CREATE SEQUENCE") {
			createSeq := simplifyCreateSequenceStatement(line)

			seqName, modifier := removeAccessModifier(strings.Split(line, " ")[2])
			if seqName[len(seqName)-1] == ';' {
				seqName = seqName[:len(seqName)-1]
			}

			if strings.Contains(lines[i+1], "ALTER SEQUENCE ") && strings.Contains(lines[i+1], seqName) {
				alterSeq := lines[i+1]
				ownedBy := strings.Split(alterSeq, " ")[5]
				var tableName string
				if strings.Count(ownedBy, ".") > 1 {
					tableName = strings.SplitAfterN(ownedBy, ".", 2)[1]
					tableName = strings.Split(tableName, ".")[0]
				} else {
					tableName = strings.Split(ownedBy, ".")[0]
				}

				sequence := &Sequence{
					Create:   createSeq,
					Relation: alterSeq,
				}
				if len(modifier) > 0 {
					sequence.Create = strings.Replace(createSeq, modifier+".", "", -1)
					sequence.Relation = strings.Replace(alterSeq, modifier+".", "", -1)
				}

				if table, ok := tables[tableName]; ok {
					table.Sequences = append(table.Sequences, sequence)
				} else {
					return lines, fmt.Errorf("mapping sequences - table does not exist")
				}
			} else {
				bufferLines = append(bufferLines, line)
			}
		} else if strings.Contains(line, "ALTER SEQUENCE") && strings.Contains(line, "OWNED BY") {
			continue
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines, nil
}

// StoreSequences parses sql statements and squashes them into a single create sequence statement.
// It then returns the remaining lines and sequences.
// Note: Assumes sequences with related tables have been processed and removed.
func StoreSequences(lines []string) ([]string, []string, error) {
	if len(lines) == 0 {
		return lines, nil, nil
	}

	var bufferLines, seqs []string
	for _, line := range lines {
		if strings.Contains(line, "CREATE SEQUENCE") {
			seq := simplifyCreateSequenceStatement(line)

			_, modifier := removeAccessModifier(strings.Split(line, " ")[2])
			if len(modifier) > 0 {
				seq = strings.Replace(seq, modifier+".", "", -1)
			}
			seqs = append(seqs, seq)
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines, seqs, nil
}

// MapDefaultValues parses sql statements and maps default value related statements to its column in tables
// It then returns the remaining lines
func MapDefaultValues(lines []string, tables map[string]*Table) ([]string, error) {
	if len(lines) == 0 {
		return lines, nil
	} else if len(tables) == 0 {
		return lines, fmt.Errorf("default value statements found with no mapped tables")
	}

	var bufferLines []string
	var modifier string
	for _, line := range lines {
		index := strings.Index(line, "DEFAULT")
		if index != -1 {
			tokens := strings.Split(line, " ")
			var tableName, columnName string
			if tokens[2] == "ONLY" {
				tableName, modifier = removeAccessModifier(tokens[3])
			} else {
				tableName, modifier = removeAccessModifier(tokens[2])
			}
			for i := range tokens {
				if tokens[i] == "ALTER" && tokens[i+1] == "COLUMN" {
					columnName = tokens[i+2]
					break
				}
			}
			if table, ok := tables[tableName]; ok {
				columns := table.Columns
				if column, ok := columns[columnName]; ok {
					if len(modifier) > 0 {
						column.Statement += " " + strings.Replace(line[index:len(line)-1], modifier+".", "", -1)
					} else {
						column.Statement += " " + line[index:len(line)-1]
					}
					table.Columns = columns
				} else {
					return lines, fmt.Errorf("mapping default values - column does not exist")
				}
			} else {
				return lines, fmt.Errorf("mapping default values - table does not exist")
			}
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines, nil
}

// MapConstraints parses sql statements and maps constraint related statements to its tables
// It then returns the remaining lines
func MapConstraints(lines []string, tables map[string]*Table) ([]string, error) {
	if len(lines) == 0 {
		return lines, nil
	} else if len(tables) == 0 {
		return lines, fmt.Errorf("constraint statements found with no mapped tables")
	}

	var bufferLines []string
	for _, line := range lines {
		// check for "CONSTRAINT" and not a trigger function
		index := strings.Index(line, "CONSTRAINT")
		if index != -1 && !strings.Contains(line, "CONSTRAINT TRIGGER") {
			tokens := strings.Split(line, " ")
			tableName, modifier := removeAccessModifier(tokens[3])
			constraintName := tokens[6]
			if table, ok := tables[tableName]; ok {
				if len(modifier) > 0 {
					table.Constraints[constraintName] = strings.Replace(line[index:len(line)-1], modifier+".", "", -1)
				} else {
					table.Constraints[constraintName] = line[index : len(line)-1]
				}
			} else {
				return lines, fmt.Errorf("mapping constraints - table does not exist")
			}

			// update column primary or foreign keys
			if strings.Contains(line, "PRIMARY KEY") {
				columns := strings.Split(line[strings.Index(line, "(")+1:strings.Index(line, ")")], ", ")
				for _, column := range columns {
					if currentCol, ok := tables[tableName].Columns[column]; ok {
						currentCol.IsPrimaryKey = true
					} else {
						delete(tables[tableName].Constraints, constraintName)
						return lines, fmt.Errorf("mapping constraints - column does not exist")
					}
				}
			} else if strings.Contains(line, "FOREIGN KEY") {
				columns := strings.Split(line[strings.Index(line, "(")+1:strings.Index(line, ")")], ", ")
				for _, column := range columns {
					if currentCol, ok := tables[tableName].Columns[column]; ok {
						currentCol.IsForeignKey = true
					} else {
						delete(tables[tableName].Constraints, constraintName)
						return lines, fmt.Errorf("column does not exist")
					}
				}
			}
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines, nil
}

// MapIndices parses sql statements and maps index related statements to its tables
// It then returns the remaining lines
func MapIndices(lines []string, tables map[string]*Table) ([]string, error) {
	if len(lines) == 0 {
		return lines, nil
	} else if len(tables) == 0 {
		return lines, fmt.Errorf("index statements found with no mapped tables")
	}

	var bufferLines []string
	var modifier string
	for _, line := range lines {
		if strings.Contains(line, "INDEX") {
			tokens := strings.Split(line, " ")
			tableName := ""
			for i := range tokens {
				if tokens[i] == "ON" {
					tableName, modifier = removeAccessModifier(tokens[i+1])
				}
			}
			if table, ok := tables[tableName]; ok {
				if len(modifier) > 0 {
					table.Index = append(table.Index, strings.Replace(line, modifier+".", "", -1))
				} else {
					table.Index = append(table.Index, line)
				}
			} else {
				return lines, fmt.Errorf("mapping indices - table does not exist")
			}
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines, nil
}

// SquashMultiLineStatements squashes any multi-line sql statements to a single line
func SquashMultiLineStatements(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	var bufferLines []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line[len(line)-1] != ';' {
			bufferLine := line
			j := i + 1
			for ; lines[j][len(lines[j])-1] != ';'; j++ {
				bufferLine = bufferLine + " " + strings.Trim(lines[j], " ")
			}
			if j < len(lines) {
				bufferLine = bufferLine + " " + strings.Trim(lines[j], " ")
			}
			bufferLines = append(bufferLines, bufferLine)
			i = j
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines
}

func printColumns(table *Table) {
	var primaryKeyColumns, foreignKeyColumns, columns []string
	for k, v := range table.Columns {
		if v.IsPrimaryKey {
			primaryKeyColumns = append(primaryKeyColumns, k)
		} else if v.IsForeignKey {
			foreignKeyColumns = append(foreignKeyColumns, k)
		} else {
			columns = append(columns, k)
		}
	}
	sort.Strings(primaryKeyColumns)
	sort.Strings(foreignKeyColumns)
	sort.Strings(columns)

	for i, columnName := range primaryKeyColumns {
		column := table.Columns[columnName]
		fmt.Printf("    %s %s", columnName, column.Statement)
		if i == len(primaryKeyColumns)-1 && len(foreignKeyColumns) == 0 && len(columns) == 0 && len(table.Constraints) == 0 {
			fmt.Println()
		} else {
			fmt.Print(",\n")
		}
	}

	for i, columnName := range foreignKeyColumns {
		column := table.Columns[columnName]
		fmt.Printf("    %s %s", columnName, column.Statement)
		if i == len(foreignKeyColumns)-1 && len(columns) == 0 && len(table.Constraints) == 0 {
			fmt.Println()
		} else {
			fmt.Print(",\n")
		}
	}

	for i, columnName := range columns {
		column := table.Columns[columnName]
		fmt.Printf("    %s %s", columnName, column.Statement)
		if i == len(columns)-1 && len(table.Constraints) == 0 {
			fmt.Println()
		} else {
			fmt.Print(",\n")
		}
	}
}

func printConstraints(table *Table) {
	i := 0
	constraintNames := make([]string, len(table.Constraints))
	for name := range table.Constraints {
		constraintNames[i] = name
		i++
	}
	sort.Strings(constraintNames)

	for i, constraint := range constraintNames {
		fmt.Printf("    %s", table.Constraints[constraint])
		if i == len(table.Constraints)-1 {
			fmt.Println()
		} else {
			fmt.Print(",\n")
		}
	}
}

func printTable(tableName string, table *Table) {
	fmt.Printf("CREATE TABLE %s (\n", tableName)
	printColumns(table)
	printConstraints(table)
	fmt.Println(");")
}

func getReferenceTables(tableName string, tables map[string]*Table) []string {
	var refTables []string
	for _, constraint := range tables[tableName].Constraints {
		if strings.Contains(constraint, "FOREIGN KEY") {
			tokens := strings.Split(constraint, "REFERENCES ")
			ref, _ := removeAccessModifier(tokens[1][:strings.Index(tokens[1], "(")])
			refTables = append(refTables, ref)
		}
	}
	return refTables
}

// Sort tables topologically
func sortTables(tables map[string]*Table) []string {
	nodes := make(map[string]*graph.Node)

	// create nodes
	for name := range tables {
		if name == "gorp_migrations" {
			continue
		}
		nodes[name] = &graph.Node{
			ID:       name,
			Parents:  make(map[string]*graph.Node),
			Children: make(map[string]*graph.Node),
		}
	}

	// add edges
	for id, node := range nodes {
		parents := getReferenceTables(id, tables)
		for _, parent := range parents {
			// exclude self-referencing tables
			if parent != id {
				node.Parents[parent] = nodes[parent]
				nodes[parent].Children[id] = node
			}
		}
	}

	i := 0
	sortedNodeIDs := make([]string, len(nodes))
	for len(nodes) > 0 {
		var rootNodeIDs []string
		for id, node := range nodes {
			if len(node.Parents) == 0 {
				rootNodeIDs = append(rootNodeIDs, id)
			}
		}

		if len(rootNodeIDs) == 0 {
			log.Fatal(fmt.Errorf("graph is not a dag"))
		}

		sort.Strings(rootNodeIDs)
		for _, id := range rootNodeIDs {
			for _, childNode := range nodes[id].Children {
				delete(childNode.Parents, id)
			}
			delete(nodes, id)
			sortedNodeIDs[i] = id
			i++
		}
	}
	return sortedNodeIDs
}

// StoreFunctions parses sql statements for functions.
// It then returns the remaining lines and sequences.
func StoreFunctions(lines []string) ([]string, []string, error) {
	if len(lines) == 0 {
		return lines, nil, nil
	}

	var bufferLines, functions []string
	var isProcessingFunction bool
	var accessModifier string
	for _, line := range lines {
		if strings.Contains(line, "CREATE FUNCTION") {
			isProcessingFunction = true

			_, accessModifier = removeAccessModifier(strings.Split(line, " ")[2])
			if len(accessModifier) > 0 {
				line = strings.Replace(line, accessModifier+".", "", -1)
			}

			functions = append(functions, line)
		} else if isProcessingFunction {
			functions = append(functions, line)

			if line == "$$;" {
				isProcessingFunction = false
			}
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines, functions, nil
}

// StoreTriggers parses sql statements for triggers and trigger functions.
// It then returns the remaining lines and sequences.
func StoreTriggers(lines []string) ([]string, []string, error) {
	if len(lines) == 0 {
		return lines, nil, nil
	}

	var bufferLines, triggers []string
	var accessModifier string
	for _, line := range lines {
		if strings.Contains(line, "CREATE TRIGGER") || strings.Contains(line, "CREATE CONSTRAINT TRIGGER") {
			_, accessModifier = removeAccessModifier(strings.Split(line, " ")[2])
			if len(accessModifier) > 0 {
				line = strings.Replace(line, accessModifier+".", "", -1)
			}

			triggers = append(triggers, line)
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines, triggers, nil
}

// PrintSchema prints the schema into palatable form in console output
func PrintSchema(tables map[string]*Table, seqs, functions, triggers []string) {
	// print independent sequences
	for _, seq := range seqs {
		fmt.Println(seq)
	}
	fmt.Println()

	// print tables
	tableNames := sortTables(tables)
	for i, tableName := range tableNames {
		table := tables[tableName]
		if len(table.Sequences) > 0 {
			for _, seq := range table.Sequences {
				fmt.Println(seq.Create)
			}
			printTable(tableName, table)
			for _, seq := range table.Sequences {
				fmt.Println(seq.Relation)
			}
		} else {
			printTable(tableName, table)
		}
		if len(table.Index) > 0 {
			for _, index := range table.Index {
				fmt.Println(index)
			}
		}
		if i < len(tableNames)-1 {
			fmt.Println()
		}
	}
	fmt.Println()

	// print functions
	for _, f := range functions {
		fmt.Println(f)
	}
	fmt.Println()

	// print triggers
	for _, tr := range triggers {
		fmt.Println(tr)
	}
}

// ReadLine is a wrapper around bufio's Reader ReadLine that returns only the line and a boolean indicating eof
func ReadLine(reader *bufio.Reader) (string, bool) {
	lineBytes, _, err := reader.ReadLine()
	if err != nil {
		if err != io.EOF {
			log.Fatal(err)
		}
		return "", true
	}
	return string(lineBytes), false
}
