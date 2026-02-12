package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

// snagTOML represents the top-level structure of a snag.toml file.
// Unknown sections are silently ignored (forward compatible).
type snagTOML struct {
	MinVersion string       `toml:"min_version"`
	Block      blockSection `toml:"block"`
}

// blockSection maps each hook phase to its own pattern list.
type blockSection struct {
	Diff   []string  `toml:"diff"`
	Msg    []string  `toml:"msg"`
	Push   *[]string `toml:"push"`
	Branch []string  `toml:"branch"`
}

// BlockConfig holds the resolved per-hook pattern lists.
// Push is nil when not explicitly set (fallback to Diff+Msg union).
type BlockConfig struct {
	Diff   []string
	Msg    []string
	Push   []string // nil = "not explicitly set" (falls back to Diff+Msg)
	Branch []string
}

// PushPatterns returns Push if explicitly set, otherwise the union of Diff and Msg.
func (bc *BlockConfig) PushPatterns() []string {
	if bc.Push != nil {
		return bc.Push
	}
	return deduplicatePatterns(append(append([]string{}, bc.Diff...), bc.Msg...))
}

// HasAnyPatterns reports whether any field has at least one pattern.
func (bc *BlockConfig) HasAnyPatterns() bool {
	return len(bc.Diff) > 0 || len(bc.Msg) > 0 || len(bc.Push) > 0 || len(bc.Branch) > 0
}

// loadSnagTOML parses a single snag.toml file. A missing file returns zero value with no error.
func loadSnagTOML(path string) (snagTOML, error) {
	var cfg snagTOML
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cfg.MinVersion != "" {
		if err := checkMinVersion(cfg.MinVersion, path); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

// checkMinVersion compares the min_version field against the running snag version.
// Returns an error if the running version is too old. Dev builds always pass.
func checkMinVersion(minVer, path string) error {
	cur := Version
	if cur == "dev" || strings.HasPrefix(cur, "dev+") {
		return nil
	}
	cur = strings.TrimPrefix(cur, "v")
	minVer = strings.TrimPrefix(minVer, "v")
	if compareSemver(cur, minVer) < 0 {
		return fmt.Errorf("%s requires snag >= %s (running %s)", path, minVer, Version)
	}
	return nil
}

// compareSemver compares two semver strings (major.minor.patch).
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// Non-numeric or missing parts are treated as 0.
func compareSemver(a, b string) int {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &av)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bv)
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

// walkConfig performs a single-pass walk from dir up to the filesystem root,
// checking for snag.toml and snag-local.toml at each level. Both are merged
// additively up the tree. Returns the resolved BlockConfig, whether any
// config was found, and any error.
func walkConfig(dir string) (*BlockConfig, bool, error) {
	bc := &BlockConfig{}
	found := false
	current := dir

	for {
		tomlPath := filepath.Join(current, "snag.toml")
		localPath := filepath.Join(current, "snag-local.toml")

		if fileExists(tomlPath) {
			if err := mergeTOML(bc, tomlPath); err != nil {
				return nil, false, err
			}
			found = true
		}
		if fileExists(localPath) {
			if err := mergeTOML(bc, localPath); err != nil {
				return nil, false, err
			}
			found = true
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return bc, found, nil
}

// fileExists reports whether path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// mergeTOML reads a snag.toml and appends its patterns into bc.
func mergeTOML(bc *BlockConfig, path string) error {
	cfg, err := loadSnagTOML(path)
	if err != nil {
		return err
	}
	bc.Diff = append(bc.Diff, cfg.Block.Diff...)
	bc.Msg = append(bc.Msg, cfg.Block.Msg...)
	if cfg.Block.Push != nil {
		merged := append([]string{}, bc.pushOrNil()...)
		merged = append(merged, *cfg.Block.Push...)
		bc.Push = merged
	}
	bc.Branch = append(bc.Branch, cfg.Block.Branch...)
	return nil
}

// pushOrNil returns bc.Push or nil if not set.
func (bc *BlockConfig) pushOrNil() []string {
	if bc.Push != nil {
		return bc.Push
	}
	return nil
}

// resolveBlockConfig builds the per-hook BlockConfig using all config sources.
//
// Precedence:
//  1. snag.toml walk (CWD → root, additive merge of snag.toml + snag-local.toml)
//  2. SNAG_PROTECTED_BRANCHES env var → always merges into Branch
//  3. Default protected branches ["main", "master"] → only when Branch is still empty
func resolveBlockConfig(cmd *cobra.Command) (*BlockConfig, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	bc, _, err := walkConfig(cwd)
	if err != nil {
		return nil, err
	}

	// Overlay SNAG_PROTECTED_BRANCHES env var into Branch.
	if env := os.Getenv("SNAG_PROTECTED_BRANCHES"); env != "" {
		for _, s := range strings.Split(env, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				bc.Branch = append(bc.Branch, s)
			}
		}
	}

	// Default protected branches when Branch is still empty.
	if len(bc.Branch) == 0 {
		bc.Branch = append([]string{}, defaultProtectedBranches...)
	}

	// Lowercase Diff/Msg/Push; preserve Branch case.
	bc.Diff = lowercaseAll(bc.Diff)
	bc.Msg = lowercaseAll(bc.Msg)
	if bc.Push != nil {
		bc.Push = lowercaseAll(bc.Push)
	}

	// Deduplicate all lists.
	bc.Diff = deduplicatePatterns(bc.Diff)
	bc.Msg = deduplicatePatterns(bc.Msg)
	if bc.Push != nil {
		bc.Push = deduplicatePatterns(bc.Push)
	}
	bc.Branch = deduplicatePatterns(bc.Branch)

	return bc, nil
}

// lowercaseAll returns a new slice with all strings lowercased.
func lowercaseAll(ss []string) []string {
	if ss == nil {
		return nil
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToLower(s)
	}
	return out
}
