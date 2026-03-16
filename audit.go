package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// violation records a single pattern match within a commit.
type violation struct {
	Kind    string // "msg" or "diff"
	Pattern string
}

// commitReport groups violations for a single commit.
type commitReport struct {
	SHA     string
	Subject string
	Matches []violation
}

func buildAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit [RANGE]",
		Short: "Scan git history for policy violations",
		Long: `Scan commits for block-pattern matches in messages and diffs.

Default range: last 50 commits (HEAD~50..HEAD).
Override with an explicit range like main..HEAD or --limit 0 for all.`,
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         runAudit,
	}
	cmd.Flags().Int("limit", 50, "max commits to scan (0 = unlimited)")
	return cmd
}

func runAudit(cmd *cobra.Command, args []string) error {
	bc, err := resolveBlockConfig(cmd)
	if err != nil {
		return err
	}
	if len(bc.Diff) == 0 && len(bc.Msg) == 0 {
		return nil
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	limit, _ := cmd.Flags().GetInt("limit")

	shas, err := auditRevList(args, limit)
	if err != nil {
		return err
	}
	if len(shas) == 0 {
		if !quiet {
			infof("no commits to scan")
		}
		return nil
	}

	if !quiet {
		infof("scanning %d commits...", len(shas))
	}

	reports := scanCommits(shas, bc)

	if !quiet {
		for _, r := range reports {
			fmt.Println()
			fmt.Printf("  %s — %q\n", shaStyle.Render(r.SHA[:7]), r.Subject)
			for _, m := range r.Matches {
				fmt.Printf("    %s match %s in commit %s\n",
					dimStyle.Render(m.Kind+":"),
					patternStyle.Render(fmt.Sprintf("%q", m.Pattern)),
					m.Kind)
			}
		}
		fmt.Println()
	}

	totalViolations := 0
	for _, r := range reports {
		totalViolations += len(r.Matches)
	}

	if totalViolations > 0 {
		infof("%d violations found in %d of %d commits", totalViolations, len(reports), len(shas))
		return fmt.Errorf("%d policy violations found", totalViolations)
	}

	infof("0 violations found in %d commits", len(shas))
	return nil
}

// auditRevList builds and runs the git rev-list command for the audit range.
func auditRevList(args []string, limit int) ([]string, error) {
	var revArgs []string
	if len(args) == 1 {
		revArgs = []string{"rev-list", args[0]}
	} else if limit == 0 {
		revArgs = []string{"rev-list", "HEAD"}
	} else {
		// Default: HEAD~N..HEAD. If the repo has fewer than N commits,
		// fall back to listing all commits.
		revArgs = []string{"rev-list", fmt.Sprintf("HEAD~%d..HEAD", limit)}
	}

	// Check if HEAD exists (repo might be empty).
	if err := exec.Command("git", "rev-parse", "--verify", "HEAD").Run(); err != nil {
		return nil, nil // empty repo, no commits
	}

	out, err := exec.Command("git", revArgs...).CombinedOutput()
	if err != nil {
		// If HEAD~N doesn't exist (fewer commits than N), list everything.
		if len(args) == 0 && limit > 0 {
			out, err = exec.Command("git", "rev-list", "HEAD").CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("git rev-list: %w\n%s", err, out)
			}
		} else {
			return nil, fmt.Errorf("git rev-list: %w\n%s", err, out)
		}
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}

	shas := strings.Split(text, "\n")

	// Apply limit cap when using fallback (all commits) with a nonzero limit.
	if len(args) == 0 && limit > 0 && len(shas) > limit {
		shas = shas[:limit]
	}

	return shas, nil
}

// scanCommits checks all commits' messages and diffs in bulk using
// batched git calls instead of per-commit forks.
func scanCommits(shas []string, bc *BlockConfig) []commitReport {
	reports := make([]commitReport, len(shas))
	shaIndex := make(map[string]int, len(shas))
	for i, sha := range shas {
		reports[i].SHA = sha
		shaIndex[sha] = i
	}

	// Batch fetch subjects and full messages in one git log call.
	// Format: <sha>\x00<subject>\x00<body>\x00\x01 per commit
	// \x01 is the record separator (%B can contain newlines).
	logArgs := []string{"log", "--format=%H%x00%s%x00%B%x00%x01", "--no-walk"}
	logArgs = append(logArgs, shas...)
	if logOut, err := exec.Command("git", logArgs...).CombinedOutput(); err == nil {
		for _, entry := range strings.Split(string(logOut), "\x01") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			parts := strings.SplitN(entry, "\x00", 3)
			if len(parts) < 3 {
				continue
			}
			sha := strings.TrimSpace(parts[0])
			idx, ok := shaIndex[sha]
			if !ok {
				continue
			}
			reports[idx].Subject = parts[1]
			if len(bc.Msg) > 0 {
				body := strings.TrimSuffix(parts[2], "\x00")
				if pattern, found := matchesPattern(body, bc.Msg); found {
					reports[idx].Matches = append(reports[idx].Matches, violation{Kind: "msg", Pattern: pattern})
				}
			}
		}
	}

	// Batch fetch diffs via git diff-tree --stdin.
	if len(bc.Diff) > 0 {
		cmd := exec.Command("git", "diff-tree", "-p", "--stdin")
		cmd.Stdin = strings.NewReader(strings.Join(shas, "\n") + "\n")
		if diffOut, err := cmd.CombinedOutput(); err == nil {
			// diff-tree --stdin output starts each commit with the SHA on its own line.
			// Split on SHA boundaries.
			chunks := splitDiffByCommit(string(diffOut), shas)
			for sha, diff := range chunks {
				idx := shaIndex[sha]
				if pattern, found := matchesPattern(stripDiffNoise(stripDiffMeta(diff)), bc.Diff); found {
					reports[idx].Matches = append(reports[idx].Matches, violation{Kind: "diff", Pattern: pattern})
				}
			}
		}
	}

	// Filter to only reports with violations.
	var result []commitReport
	for _, r := range reports {
		if len(r.Matches) > 0 {
			result = append(result, r)
		}
	}
	return result
}

// splitDiffByCommit splits combined diff-tree --stdin output into
// per-commit chunks keyed by full SHA.
func splitDiffByCommit(output string, shas []string) map[string]string {
	shaSet := make(map[string]bool, len(shas))
	for _, sha := range shas {
		shaSet[sha] = true
	}

	chunks := make(map[string]string, len(shas))
	var currentSHA string
	var buf strings.Builder

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 40 && shaSet[trimmed] {
			if currentSHA != "" {
				chunks[currentSHA] = buf.String()
			}
			currentSHA = trimmed
			buf.Reset()
			continue
		}
		if currentSHA != "" {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	if currentSHA != "" {
		chunks[currentSHA] = buf.String()
	}
	return chunks
}
