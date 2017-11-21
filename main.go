package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/psql-schema-dump-sanitiser.git/parse"
)

func main() {
	if len(os.Args)-1 == 0 {
		log.Fatal("Missing argument: \"postgres-dump-sanitiser <file>\"")
		return
	}

	// prepare file and reader
	filePath := os.Args[1]
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)

	lines := make([]string, 0)
	// read file by line till EOF
	for {
		line, eof := parse.ReadLine(reader)
		if eof {
			break
		}

		// 1. Check for redundant lines
		if parse.IsRedundant(line) {
			continue
		}
		lines = append(lines, line)
	}

	// 3. Group and map table statements
	tables, lines := parse.MapTables(lines)

	// 4. Squash any multi-line statements to single line
	lines = parse.SquashMultiLineStatements(lines)

	// 5. Squash sequence statements into create sequence statements and map to tables
	lines = parse.MapSequences(lines, tables)

	// 6. Add default values to columns
	lines = parse.MapDefaultValues(lines, tables)

	// 7. Map constraint statements to tables
	lines = parse.MapConstraints(lines, tables)

	// 8. Map index statements to tables
	lines = parse.MapIndices(lines, tables)

	if len(lines) != 0 {
		log.Fatal(fmt.Errorf("%d unprocessed lines remaining", len(lines)))
	}

	// 9. Print
	parse.PrintSchema(tables)
}
