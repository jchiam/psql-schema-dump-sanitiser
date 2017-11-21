package main

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"strings"

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
	var bufferLines []string
	// read file by line till EOF
	for {
		line, eof := parse.ReadLine(reader)
		if eof {
			break
		}
		if len(line) == 0 {
			continue
		}
		tokens := strings.Split(line, " ")

		// 1. Skip lines that start with "--", "SET"
		if tokens[0] == "--" || tokens[0] == "SET" {
			continue
		}
		// 2. Remove extension and owner statements
		if strings.Contains(line, "EXTENSION") || strings.Contains(line, "OWNER") {
			continue
		}
		// 3. Squash sequence statements
		if strings.Contains(line, "CREATE SEQUENCE") {
			seqLine := line
			for line[len(line)-1] != ';' {
				line, eof = parse.ReadLine(reader)
				seqLine = seqLine + " " + strings.Trim(line, " ")
			}

			seqStartIndex := strings.Index(seqLine, "START WITH 1")
			if seqStartIndex != -1 {
				seqLine = seqLine[:seqStartIndex] + seqLine[seqStartIndex+len("START WITH 1"):]
			}
			seqIncrIndex := strings.Index(seqLine, "INCREMENT BY 1")
			if seqIncrIndex != -1 {
				seqLine = seqLine[:seqStartIndex] + seqLine[seqIncrIndex+len("INCREMENT BY 1"):]
			}
			seqMinIndex := strings.Index(seqLine, "NO MINVALUE")
			if seqMinIndex != -1 {
				seqLine = seqLine[:seqIncrIndex] + seqLine[seqIncrIndex+len("NO MINVALUE"):]
			}
			seqMaxIndex := strings.Index(seqLine, "NO MAXVALUE")
			if seqMaxIndex != -1 {
				seqLine = seqLine[:seqMaxIndex] + seqLine[seqMaxIndex+len("NO MAXVALUE"):]
			}
			seqCacheIndex := strings.Index(seqLine, "CACHE 1")
			if seqCacheIndex != -1 {
				seqLine = seqLine[:seqCacheIndex] + seqLine[seqCacheIndex+len("CACHE 1"):]
			}

			multipleWhiteSpaceExp := regexp.MustCompile(`[\s]{2,}`)
			seqLine = multipleWhiteSpaceExp.ReplaceAllString(seqLine, " ")

			spacesBeforeSemicolon := regexp.MustCompile(`[\s]{1,};`)
			seqLine = spacesBeforeSemicolon.ReplaceAllString(seqLine, ";")

			line = seqLine
		}

		lines = append(lines, line)
	}

	// 4. Group and map table statements
	tables, lines := parse.MapTables(lines)

	// 5. Squash sequence statements into create sequence statements and map to tables
	lines = parse.MapSequences(lines, tables)

	// 6. Squash remaining statements to single line
	bufferLines = make([]string, 0)
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
	lines = bufferLines

	// 7. Add default values to columns
	lines = parse.MapDefaultValues(lines, tables)

	// 8. Map constraint statements to tables
	bufferLines = make([]string, 0)
	for _, line := range lines {
		index := strings.Index(line, "CONSTRAINT")
		if index != -1 {
			tokens := strings.Split(line, " ")
			tableName := tokens[3]
			constraints := tables[tableName].Constraints
			tables[tableName].Constraints = append(constraints, line[index:len(line)-1])
		} else {
			bufferLines = append(bufferLines, line)
		}
	}
	lines = bufferLines

	// 9. Map index statements to tables
	lines = parse.MapIndices(lines, tables)

	// 10. Print
	parse.PrintSchema(tables)
}
