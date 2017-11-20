package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
)

type table struct {
	Columns     map[string]string
	Constraints []string
	Sequence    string
}

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
		line, eof := readLine(reader)
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
				line, eof = readLine(reader)
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
	bufferLines = make([]string, 0)
	tables := make(map[string]*table)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "CREATE TABLE") {
			tableName := strings.Split(line, " ")[2]
			table := table{
				Columns:     make(map[string]string),
				Constraints: make([]string, 0),
				Sequence:    "",
			}

			j := i + 1
			for ; lines[j][len(lines[j])-1] != ';'; j++ {
				columnLine := strings.Trim(lines[j], " ")
				spaceIndex := strings.Index(columnLine, " ")
				columnName := columnLine[:spaceIndex]
				table.Columns[columnName] = columnLine[spaceIndex+1:]
			}

			tables[tableName] = &table
			i = j
		} else {
			bufferLines = append(bufferLines, line)
		}
	}
	lines = bufferLines

	// 5. Combine create sequence and alter sequence owned by
	bufferLines = make([]string, 0)
	for i, line := range lines {
		if strings.Contains(line, "CREATE SEQUENCE") {
			seqName := strings.Split(line, " ")[2]
			seqName = seqName[:len(seqName)-1]
			if strings.Contains(lines[i+1], "ALTER SEQUENCE "+seqName) {
				ownedBy := strings.Split(lines[i+1], " ")[5]
				seqTable := strings.Split(ownedBy, ".")[0]
				seqColumn := strings.Split(ownedBy, ".")[1]
				seqColumn = seqColumn[:len(seqColumn)-1]
				tables[seqTable].Sequence = line[:len(line)-1] + " OWNED BY " + ownedBy + ";"
			}
		} else if strings.Contains(line, "ALTER SEQUENCE") && strings.Contains(line, "OWNED BY") {
			continue
		} else {
			bufferLines = append(bufferLines, line)
		}
	}
	lines = bufferLines

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

	for _, line := range lines {
		fmt.Println(line)
	}
}

func readLine(reader *bufio.Reader) (string, bool) {
	lineBytes, _, err := reader.ReadLine()
	if err != nil {
		if err != io.EOF {
			log.Fatal(err)
		}
		return "", true
	}
	return string(lineBytes), false
}
