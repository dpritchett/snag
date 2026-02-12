package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func buildConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "config",
		Short:        "Show resolved block patterns and their sources",
		SilenceUsage: true,
		RunE:         runConfig,
	}
}

// configSource pairs a source label with the patterns it contributes.
type configSource struct {
	Label  string
	Kind   string // "toml", "env", "default"
	Diff   []string
	Msg    []string
	Push   *[]string // nil = not set
	Branch []string
}

func runConfig(cmd *cobra.Command, args []string) error {
	sources, err := collectSources(cmd)
	if err != nil {
		return err
	}

	if len(sources) == 0 {
		fmt.Fprintln(os.Stderr, hintStyle.Render("  no snag config found"))
		return nil
	}

	for i, src := range sources {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(hintStyle.Render("# " + src.Label))

		switch src.Kind {
		case "toml":
			printSection("diff", src.Diff)
			printSection("msg", src.Msg)
			if src.Push != nil {
				printSection("push", *src.Push)
			}
			printSection("branch", src.Branch)
		case "env":
			printSection("branch", src.Branch)
		case "default":
			printSection("branch", src.Branch)
		}
	}

	// Show the effective push behavior if no source explicitly set push.
	hasPush := false
	for _, src := range sources {
		if src.Push != nil {
			hasPush = true
			break
		}
	}
	if !hasPush {
		fmt.Println()
		fmt.Println(hintStyle.Render("# push: inherits union of diff + msg"))
	}

	return nil
}

func printSection(name string, patterns []string) {
	if len(patterns) == 0 {
		return
	}
	fmt.Printf("  %-8s %s\n", name+":", strings.Join(patterns, ", "))
}

// collectSources gathers config sources with provenance for display.
func collectSources(cmd *cobra.Command) ([]configSource, error) {
	var sources []configSource

	fileSources, err := walkConfigSources()
	if err != nil {
		return nil, err
	}
	sources = append(sources, fileSources...)

	// SNAG_PROTECTED_BRANCHES env var
	if env := os.Getenv("SNAG_PROTECTED_BRANCHES"); env != "" {
		var branches []string
		for _, s := range strings.Split(env, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				branches = append(branches, s)
			}
		}
		if len(branches) > 0 {
			sources = append(sources, configSource{
				Label:  "SNAG_PROTECTED_BRANCHES",
				Kind:   "env",
				Branch: branches,
			})
		}
	}

	// Default protected branches (only if no branch patterns from any source)
	hasBranch := false
	for _, src := range sources {
		if len(src.Branch) > 0 {
			hasBranch = true
			break
		}
	}
	if !hasBranch {
		sources = append(sources, configSource{
			Label:  "defaults",
			Kind:   "default",
			Branch: defaultProtectedBranches,
		})
	}

	return sources, nil
}

// walkConfigSources walks from CWD to root collecting config files with paths.
func walkConfigSources() ([]configSource, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	var sources []configSource
	current := cwd

	for {
		tomlPath := filepath.Join(current, "snag.toml")
		localPath := filepath.Join(current, "snag-local.toml")

		if fileExists(tomlPath) {
			if src, err := tomlSource(tomlPath); err != nil {
				return nil, err
			} else if src != nil {
				sources = append(sources, *src)
			}
		}
		if fileExists(localPath) {
			if src, err := tomlSource(localPath); err != nil {
				return nil, err
			} else if src != nil {
				sources = append(sources, *src)
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return sources, nil
}

func tomlSource(path string) (*configSource, error) {
	cfg, err := loadSnagTOML(path)
	if err != nil {
		return nil, err
	}
	abs, _ := filepath.Abs(path)
	src := &configSource{
		Label:  abs,
		Kind:   "toml",
		Diff:   cfg.Block.Diff,
		Msg:    cfg.Block.Msg,
		Push:   cfg.Block.Push,
		Branch: cfg.Block.Branch,
	}
	// Skip empty sources
	if len(src.Diff) == 0 && len(src.Msg) == 0 && src.Push == nil && len(src.Branch) == 0 {
		return nil, nil
	}
	return src, nil
}
