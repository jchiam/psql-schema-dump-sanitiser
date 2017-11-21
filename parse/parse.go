package parse

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"sort"
)

// Table is the struct containing logical aspects of a psql table's structure
type Table struct {
	Columns     map[string]string
	Constraints []string
	Sequence    string
	Index       string
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

// PrintSchema prints the schema into palatable form in console output
func PrintSchema(tables map[string]*Table) {
	tableNames := make([]string, len(tables)-1)
	i := 0
	for k := range tables {
		if k == "gorp_migrations" {
			continue
		}
		tableNames[i] = k
		i++
	}
	sort.Strings(tableNames)

	for _, tableName := range tableNames {
		table := tables[tableName]
		fmt.Printf("CREATE TABLE %s (\n", tableName)
		i = 0
		for columnName, column := range table.Columns {
			fmt.Printf("    %s %s", columnName, column)
			if i == len(table.Columns)-1 && len(table.Constraints) == 0 {
				fmt.Println()
			} else {
				fmt.Print(",\n")
			}
			i++
		}
		i = 0
		for _, constraint := range table.Constraints {
			fmt.Printf("    %s", constraint)
			if i == len(table.Constraints)-1 {
				fmt.Println()
			} else {
				fmt.Print(",\n")
			}
			i++
		}
		fmt.Println(");")
		if len(table.Sequence) > 0 {
			fmt.Println(table.Sequence)
		}
		if len(table.Index) > 0 {
			fmt.Println(table.Index)
		}
		fmt.Println()
	}
}
