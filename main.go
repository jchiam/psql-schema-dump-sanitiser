package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/jchiam/psql-schema-dump-sanitiser/parse"
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
	defer func() {
		cerr := file.Close()
		if cerr != nil {
			log.Fatal(cerr)
		}
	}()
	reader := bufio.NewReader(file)

	var lines []string
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

	// 2. Group and map table statements
	tables, lines := parse.MapTables(lines)

	// 3. Squash any multi-line statements to single line
	lines = parse.SquashMultiLineStatements(lines)

	// 4. Squash sequence statements into create sequence statements and map to tables
	lines, err = parse.MapSequences(lines, tables)
	if err != nil {
		log.Fatal(err)
	}

	// 5. Store sequences not owned by table columns
	lines, seqs, err := parse.StoreSequences(lines)
	if err != nil {
		log.Fatal(err)
	}

	// 6. Add default values to columns
	lines, err = parse.MapDefaultValues(lines, tables)
	if err != nil {
		log.Fatal(err)
	}

	// 7. Map constraint statements to tables
	lines, err = parse.MapConstraints(lines, tables)
	if err != nil {
		log.Fatal(err)
	}

	// 8. Map index statements to tables
	lines, err = parse.MapIndices(lines, tables)
	if err != nil {
		log.Fatal(err)
	}

	if len(lines) != 0 {
		log.Fatal(fmt.Errorf("%d unprocessed lines remaining", len(lines)))
	}

	// 9. Print
	parse.PrintSchema(tables, seqs)
}
