package output

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// WriteFields writes key=value pairs to outputFile (append mode) or stdout if outputFile is empty.
// Keys are sorted for deterministic output.
// Returns error if file write fails.
func WriteFields(outputFile string, fields map[string]string) error {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build output lines
	var lines []string
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, fields[k]))
	}
	output := strings.Join(lines, "\n")
	if len(output) > 0 {
		output += "\n"
	}

	// Write to file or stdout
	if outputFile != "" {
		// Append to file
		f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open output file: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(output); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
	} else {
		// Write to stdout
		fmt.Print(output)
	}

	return nil
}
