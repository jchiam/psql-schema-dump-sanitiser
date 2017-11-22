package parse

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/gyuho/goraph"
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
	Constraints []string
	Sequences   []*Sequence
	Index       string
}

// IsRedundant checks if line is a redundant sql statement or comment
func IsRedundant(line string) bool {
	if len(line) == 0 {
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

	return false
}

// MapTables parses sql statements and returns a map of Table structs containing information of table's structure
// and the remaining unprocessed lines
func MapTables(lines []string) (map[string]*Table, []string) {
	tables := make(map[string]*Table)
	if len(lines) == 0 {
		return tables, lines
	}

	bufferLines := make([]string, 0)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "CREATE TABLE") {
			tableName := strings.Split(line, " ")[2]
			table := Table{
				Columns:     make(map[string]*Column),
				Constraints: make([]string, 0),
				Sequences:   make([]*Sequence, 0),
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

// MapSequences parses sql statements and squashes them into a single create sequence statement.
// It then returns the remaining lines.
// Note: Assumes relation statement is always after create statement.
func MapSequences(lines []string, tables map[string]*Table) []string {
	if len(lines) == 0 {
		return lines
	} else if len(tables) == 0 {
		log.Fatal(fmt.Errorf("sequence statements found with no mapped tables"))
	}

	bufferLines := make([]string, 0)
	for i, line := range lines {
		if strings.Contains(line, "CREATE SEQUENCE") {
			createSeq := simplifyCreateSequenceStatement(line)

			seqName := strings.Split(line, " ")[2]
			if seqName[len(seqName)-1] == ';' {
				seqName = seqName[:len(seqName)-1]
			}

			if strings.Contains(lines[i+1], "ALTER SEQUENCE "+seqName) {
				alterSeq := lines[i+1]
				ownedBy := strings.Split(alterSeq, " ")[5]
				seqTable := strings.Split(ownedBy, ".")[0]

				sequence := &Sequence{
					Create:   createSeq,
					Relation: alterSeq,
				}
				tables[seqTable].Sequences = append(tables[seqTable].Sequences, sequence)
			}
		} else if strings.Contains(line, "ALTER SEQUENCE") && strings.Contains(line, "OWNED BY") {
			continue
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines
}

// MapDefaultValues parses sql statements and maps default value related statements to its column in tables
// It then returns the remaining lines
func MapDefaultValues(lines []string, tables map[string]*Table) []string {
	if len(lines) == 0 {
		return lines
	} else if len(tables) == 0 {
		log.Fatal(fmt.Errorf("index statements found with no mapped tables"))
	}

	bufferLines := make([]string, 0)
	for _, line := range lines {
		index := strings.Index(line, "DEFAULT nextval")
		if index != -1 {
			tokens := strings.Split(line, " ")
			var tableName, columnName string
			if tokens[2] == "ONLY" {
				tableName = tokens[3]
			} else {
				tableName = tokens[2]
			}
			for i := range tokens {
				if tokens[i] == "ALTER" && tokens[i+1] == "COLUMN" {
					columnName = tokens[i+2]
					break
				}
			}
			columns := tables[tableName].Columns
			columns[columnName].Statement += " " + line[index:len(line)-1]
			tables[tableName].Columns = columns
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines
}

// MapConstraints parses sql statements and maps constraint related statements to its tables
// It then returns the remaining lines
func MapConstraints(lines []string, tables map[string]*Table) []string {
	if len(lines) == 0 {
		return lines
	} else if len(tables) == 0 {
		log.Fatal(fmt.Errorf("index statements found with no mapped tables"))
	}

	bufferLines := make([]string, 0)
	for _, line := range lines {
		index := strings.Index(line, "CONSTRAINT")
		if index != -1 {
			tokens := strings.Split(line, " ")
			tableName := tokens[3]
			constraints := tables[tableName].Constraints
			tables[tableName].Constraints = append(constraints, line[index:len(line)-1])

			// update column primary or foreign keys
			if strings.Index(line, "PRIMARY KEY") != -1 {
				columns := strings.Split(line[strings.Index(line, "(")+1:strings.Index(line, ")")], ", ")
				for _, column := range columns {
					tables[tableName].Columns[column].IsPrimaryKey = true
				}
			} else if strings.Index(line, "FOREIGN KEY") != -1 {
				columns := strings.Split(line[strings.Index(line, "(")+1:strings.Index(line, ")")], ", ")
				for _, column := range columns {
					tables[tableName].Columns[column].IsForeignKey = true
				}
			}
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines
}

// MapIndices parses sql statements and maps index related statements to its tables
// It then returns the remaining lines
func MapIndices(lines []string, tables map[string]*Table) []string {
	if len(lines) == 0 {
		return lines
	} else if len(tables) == 0 {
		log.Fatal(fmt.Errorf("index statements found with no mapped tables"))
	}

	bufferLines := make([]string, 0)
	for _, line := range lines {
		if strings.Contains(line, "INDEX") {
			tokens := strings.Split(line, " ")
			tableName := ""
			for i := range tokens {
				if tokens[i] == "ON" {
					tableName = tokens[i+1]
				}
			}
			tables[tableName].Index = line
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines
}

// SquashMultiLineStatements squashes any multi-line sql statements to a single line
func SquashMultiLineStatements(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	bufferLines := make([]string, 0)
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
	primaryKeyColumns := make([]string, 0)
	foreignKeyColumns := make([]string, 0)
	columns := make([]string, 0)
	for k, v := range table.Columns {
		if v.IsPrimaryKey {
			primaryKeyColumns = append(primaryKeyColumns, k)
		} else if v.IsForeignKey {
			foreignKeyColumns = append(foreignKeyColumns, k)
		} else {
			columns = append(columns, k)
		}
	}
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
	for _, constraint := range table.Constraints {
		fmt.Printf("    %s", constraint)
		if i == len(table.Constraints)-1 {
			fmt.Println()
		} else {
			fmt.Print(",\n")
		}
		i++
	}
}

func printTable(tableName string, table *Table) {
	fmt.Printf("CREATE TABLE %s (\n", tableName)
	printColumns(table)
	printConstraints(table)
	fmt.Println(");")
}

func getReferenceTable(tableName string, tables map[string]*Table) []string {
	refTables := make([]string, 0)
	for _, constraint := range tables[tableName].Constraints {
		if strings.Contains(constraint, "FOREIGN KEY") {
			tokens := strings.Split(constraint, "REFERENCES ")
			refTables = append(refTables, tokens[1][:strings.Index(tokens[1], "(")])
		}
	}
	return refTables
}

// Sort tables topologically
func sortTables(tables map[string]*Table) []string {
	graph := goraph.NewGraph()
	nodeIDs := make(map[string]goraph.ID)

	// create nodes and populate graph
	for k := range tables {
		if k == "gorp_migrations" {
			continue
		}
		node := goraph.NewNode(k)
		nodeIDs[k] = node.ID()
		graph.AddNode(goraph.NewNode(k))
	}

	// populate graph with edges
	nodes := graph.GetNodes()
	for id := range nodes {
		refTables := getReferenceTable(id.String(), tables)
		for _, ref := range refTables {
			graph.AddEdge(nodeIDs[ref], id, 0)
		}
	}

	IDs, isDAG := goraph.TopologicalSort(graph)
	if !isDAG {
		log.Fatal(fmt.Errorf("error in sorting tables"))
	}
	sortedTableNames := make([]string, len(IDs))
	i := 0
	for _, id := range IDs {
		sortedTableNames[i] = id.String()
		i++
	}
	return sortedTableNames
}

// PrintSchema prints the schema into palatable form in console output
func PrintSchema(tables map[string]*Table) {
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
			fmt.Println(table.Index)
		}
		if i < len(tableNames)-1 {
			fmt.Println()
		}
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
