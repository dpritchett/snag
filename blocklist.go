package main

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

// loadBlocklist reads patterns from a file, one per line.
// Blank lines and lines starting with # are skipped.
// All patterns are lowercased. A missing file returns (nil, nil).
func loadBlocklist(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, strings.ToLower(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return patterns, nil
}

// matchesBlocklist checks whether text contains any of the given patterns.
// Comparison is case-insensitive. Returns the matched pattern and true on
// the first hit, or ("", false) if nothing matches.
func matchesBlocklist(text string, patterns []string) (string, bool) {
	lower := strings.ToLower(text)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return p, true
		}
	}
	return "", false
}

// isTrailerLine reports whether line is a valid Git trailer (Key: Value).
// The key must have no spaces, no leading whitespace, and be followed by ": ".
func isTrailerLine(line string) bool {
	if line == "" {
		return false
	}
	if strings.TrimLeft(line, " \t") != line {
		return false
	}
	idx := strings.Index(line, ": ")
	if idx < 1 {
		return false
	}
	key := line[:idx]
	return !strings.Contains(key, " ")
}
