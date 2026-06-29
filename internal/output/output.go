package output

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// WriteFields writes key=value pairs to outputFile (append mode) or stdout if outputFile is empty.
// Keys are sorted for deterministic output.
// Multiline values are written in the heredoc format required by GitHub Actions GITHUB_OUTPUT.
// Returns error if file write fails.
func WriteFields(outputFile string, fields map[string]string) (err error) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	const delimiter = "_GitHubActionsFileCommandDelimeter_"
	var sb strings.Builder
	for _, k := range keys {
		v := fields[k]
		if strings.Contains(v, "\n") {
			// Multiline: use heredoc format required by GitHub Actions
			fmt.Fprintf(&sb, "%s<<%s\n%s\n%s\n", k, delimiter, v, delimiter)
		} else {
			fmt.Fprintf(&sb, "%s=%s\n", k, v)
		}
	}

	// Write to file or stdout
	if outputFile != "" {
		// Append to file
		var f *os.File
		f, err = os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open output file: %w", err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil && err == nil {
				err = fmt.Errorf("close output file: %w", cerr)
			}
		}()
		_, err = f.WriteString(sb.String())
		if err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
	} else {
		// Write to stdout
		fmt.Print(sb.String())
	}

	return nil
}
