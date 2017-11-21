package parse

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
)

// Table is the struct containing logical aspects of a psql table's structure
type Table struct {
	Columns     map[string]string
	Constraints []string
	Sequence    string
	Index       string
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
				Columns:     make(map[string]string),
				Constraints: make([]string, 0),
			}

			j := i + 1
			for ; lines[j][len(lines[j])-1] != ';'; j++ {
				columnLine := strings.Trim(lines[j], " ")
				spaceIndex := strings.Index(columnLine, " ")
				columnName := columnLine[:spaceIndex]
				if columnLine[len(columnLine)-1] == ',' {
					table.Columns[columnName] = columnLine[spaceIndex+1 : len(columnLine)-1]
				} else {
					table.Columns[columnName] = columnLine[spaceIndex+1:]
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

// PrintSchema prints the schema into palatable form in console output
func PrintSchema(tables map[string]*Table) {
	tableNames := make([]string, 0)
	for k := range tables {
		if k == "gorp_migrations" {
			continue
		}
		tableNames = append(tableNames, k)
	}
	sort.Strings(tableNames)

	for i, tableName := range tableNames {
		table := tables[tableName]
		fmt.Printf("CREATE TABLE %s (\n", tableName)
		j := 0
		for columnName, column := range table.Columns {
			fmt.Printf("    %s %s", columnName, column)
			if j == len(table.Columns)-1 && len(table.Constraints) == 0 {
				fmt.Println()
			} else {
				fmt.Print(",\n")
			}
			j++
		}
		j = 0
		for _, constraint := range table.Constraints {
			fmt.Printf("    %s", constraint)
			if j == len(table.Constraints)-1 {
				fmt.Println()
			} else {
				fmt.Print(",\n")
			}
			j++
		}
		fmt.Println(");")
		if len(table.Sequence) > 0 {
			fmt.Println(table.Sequence)
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
