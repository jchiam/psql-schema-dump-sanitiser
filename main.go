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
		fmt.Println(line)
	}

	// for _, line := range lines {
	// 	fmt.Println(line)
	// }
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
