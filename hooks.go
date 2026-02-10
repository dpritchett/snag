package main

import "github.com/spf13/cobra"

// Hook describes a single policy check that snag can run.
type Hook struct {
	Name   string                                      // "diff", "msg", "push"
	Use    string                                      // cobra Use string
	Short  string                                      // cobra Short description
	Args   cobra.PositionalArgs                        // nil = no positional args
	RunE   func(*cobra.Command, []string) error        // the check itself
	TestFn func(*cobra.Command, string, []string) bool // demo/test scenario
}

var hooks = []Hook{
	{
		Name:   "diff",
		Use:    "diff",
		Short:  "Check staged diff against policies",
		RunE:   runDiff,
		TestFn: testDiff,
	},
	{
		Name:   "msg",
		Use:    "msg FILE",
		Short:  "Check commit message against policies",
		Args:   cobra.ExactArgs(1),
		RunE:   runMsg,
		TestFn: testMsg,
	},
	{
		Name:   "push",
		Use:    "push",
		Short:  "Check pre-push policies",
		RunE:   runPush,
		TestFn: testPush,
	},
}
