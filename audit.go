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

	var reports []commitReport
	for _, sha := range shas {
		report := scanCommit(sha, bc)
		if len(report.Matches) > 0 {
			reports = append(reports, report)
		}
	}

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

// scanCommit checks a single commit's message and diff against patterns.
func scanCommit(sha string, bc *BlockConfig) commitReport {
	report := commitReport{SHA: sha}

	// Get subject line for display.
	subOut, _ := exec.Command("git", "log", "-1", "--format=%s", sha).CombinedOutput()
	report.Subject = strings.TrimSpace(string(subOut))

	// Check commit message against msg patterns.
	if len(bc.Msg) > 0 {
		msgOut, err := exec.Command("git", "log", "-1", "--format=%B", sha).CombinedOutput()
		if err == nil {
			if pattern, found := matchesBlocklist(string(msgOut), bc.Msg); found {
				report.Matches = append(report.Matches, violation{Kind: "msg", Pattern: pattern})
			}
		}
	}

	// Check commit diff against diff patterns.
	if len(bc.Diff) > 0 {
		diffOut, err := exec.Command("git", "diff-tree", "-p", sha).CombinedOutput()
		if err == nil {
			if pattern, found := matchesBlocklist(stripDiffNoise(stripDiffMeta(string(diffOut))), bc.Diff); found {
				report.Matches = append(report.Matches, violation{Kind: "diff", Pattern: pattern})
			}
		}
	}

	return report
}
