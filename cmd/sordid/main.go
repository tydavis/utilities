package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
)

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines writes the lines to the given file.
func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}

func main() {
	var file string
	flag.StringVar(&file, "f", "", "File to sort")
	flag.Parse()

	if file == "" {
		if len(flag.Args()) > 0 && os.Args[1] != "" {
			file = os.Args[1]
		} else {
			flag.PrintDefaults()
			os.Exit(2)
		}
	}

	lines, err := readLines(file)
	if err != nil {
		log.Fatalf("failed to read: %s", err)
	}

	sort.Strings(lines)

	if err := writeLines(lines, file); err != nil {
		log.Fatalf("failed to write file: %s", err)
	}

}
