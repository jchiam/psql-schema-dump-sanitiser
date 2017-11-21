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

// MapSequences parses sql statements and squashes them into a single create sequence statement.
// It then returns the remaining lines
func MapSequences(lines []string, tables map[string]*Table) []string {
	if len(lines) == 0 {
		return lines
	} else if len(tables) == 0 {
		log.Fatal(fmt.Errorf("sequence statements found with no mapped tables"))
	}

	bufferLines := make([]string, 0)
	for i, line := range lines {
		if strings.Contains(line, "CREATE SEQUENCE") {
			seqName := strings.Split(line, " ")[2]
			seqName = seqName[:len(seqName)-1]
			if strings.Contains(lines[i+1], "ALTER SEQUENCE "+seqName) {
				ownedBy := strings.Split(lines[i+1], " ")[5]
				seqTable := strings.Split(ownedBy, ".")[0]
				tables[seqTable].Sequence = line[:len(line)-1] + " OWNED BY " + ownedBy
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
			columns[columnName] += " " + line[index:len(line)-1]
			tables[tableName].Columns = columns
		} else {
			bufferLines = append(bufferLines, line)
		}
	}

	return bufferLines
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
