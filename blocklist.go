package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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

// walkBlocklists walks from dir up to the filesystem root, loading every
// .blocklist file it finds and merging the patterns.
func walkBlocklists(dir string) ([]string, error) {
	var all []string
	current := dir
	for {
		p := filepath.Join(current, ".blocklist")
		patterns, err := loadBlocklist(p)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", p, err)
		}
		all = append(all, patterns...)

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return all, nil
}

// loadEnvBlocklist parses the SNAG_BLOCKLIST environment variable.
// Patterns can be separated by newlines or colons (or both).
// Comments (#) and blank entries are skipped. All patterns are lowercased.
func loadEnvBlocklist() []string {
	val := os.Getenv("SNAG_BLOCKLIST")
	if val == "" {
		return nil
	}
	// Normalize colons to newlines so both delimiters work.
	val = strings.ReplaceAll(val, ":", "\n")
	var patterns []string
	for _, line := range strings.Split(val, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, strings.ToLower(line))
	}
	return patterns
}

// deduplicatePatterns removes duplicate patterns, preserving first-occurrence order.
func deduplicatePatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for _, p := range patterns {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}

// resolvePatterns builds the final pattern list for a subcommand.
// If --blocklist was explicitly passed, only that file is used.
// Otherwise, .blocklist files are collected by walking up from CWD.
// The SNAG_BLOCKLIST env var is always merged on top.
func resolvePatterns(cmd *cobra.Command) ([]string, error) {
	var patterns []string

	if cmd.Flags().Changed("blocklist") {
		path, _ := cmd.Flags().GetString("blocklist")
		p, err := loadBlocklist(path)
		if err != nil {
			return nil, fmt.Errorf("loading blocklist: %w", err)
		}
		patterns = p
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting working directory: %w", err)
		}
		p, err := walkBlocklists(cwd)
		if err != nil {
			return nil, err
		}
		patterns = p
	}

	patterns = append(patterns, loadEnvBlocklist()...)
	return deduplicatePatterns(patterns), nil
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
