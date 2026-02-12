package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadSnagTOML(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		cfg, err := loadSnagTOML("/no/such/snag.toml")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(cfg.Block.Diff) != 0 {
			t.Fatalf("expected empty block.diff, got %v", cfg.Block.Diff)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "snag.toml")
		os.WriteFile(path, []byte(""), 0644)
		cfg, err := loadSnagTOML(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Block.Diff) != 0 {
			t.Fatalf("expected empty, got %v", cfg.Block.Diff)
		}
	})

	t.Run("valid block section", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "snag.toml")
		os.WriteFile(path, []byte(`
[block]
diff = ["HACK", "DO NOT MERGE"]
msg  = ["HACK", "WIP"]
push = ["SECRET_KEY"]
branch = ["main", "master", "release/*"]
`), 0644)
		cfg, err := loadSnagTOML(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Block.Diff) != 2 {
			t.Errorf("diff: got %d, want 2", len(cfg.Block.Diff))
		}
		if len(cfg.Block.Msg) != 2 {
			t.Errorf("msg: got %d, want 2", len(cfg.Block.Msg))
		}
		if cfg.Block.Push == nil || len(*cfg.Block.Push) != 1 {
			t.Errorf("push: got %v, want [SECRET_KEY]", cfg.Block.Push)
		}
		if len(cfg.Block.Branch) != 3 {
			t.Errorf("branch: got %d, want 3", len(cfg.Block.Branch))
		}
	})

	t.Run("partial keys", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "snag.toml")
		os.WriteFile(path, []byte(`
[block]
diff = ["TODO"]
`), 0644)
		cfg, err := loadSnagTOML(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Block.Diff) != 1 || cfg.Block.Diff[0] != "TODO" {
			t.Errorf("diff: got %v, want [TODO]", cfg.Block.Diff)
		}
		if len(cfg.Block.Msg) != 0 {
			t.Errorf("msg: got %v, want empty", cfg.Block.Msg)
		}
		if cfg.Block.Push != nil {
			t.Errorf("push: got %v, want nil", cfg.Block.Push)
		}
	})

	t.Run("unknown sections ignored", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "snag.toml")
		os.WriteFile(path, []byte(`
[require]
checks = ["lint"]

[block]
diff = ["TODO"]

[identity]
email = "test@example.com"
`), 0644)
		cfg, err := loadSnagTOML(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Block.Diff) != 1 {
			t.Errorf("diff: got %d, want 1", len(cfg.Block.Diff))
		}
	})

	t.Run("malformed TOML", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "snag.toml")
		os.WriteFile(path, []byte(`not valid [ toml = `), 0644)
		_, err := loadSnagTOML(path)
		if err == nil {
			t.Fatal("expected error for malformed TOML")
		}
	})
}

func TestWalkConfig(t *testing.T) {
	t.Run("single snag.toml", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["TODO"]
msg  = ["WIP"]
`), 0644)
		bc, found, err := walkConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected found=true")
		}
		if len(bc.Diff) != 1 || bc.Diff[0] != "TODO" {
			t.Errorf("diff: got %v, want [TODO]", bc.Diff)
		}
		if len(bc.Msg) != 1 || bc.Msg[0] != "WIP" {
			t.Errorf("msg: got %v, want [WIP]", bc.Msg)
		}
	})

	t.Run("parent and child snag.toml merge", func(t *testing.T) {
		parent := t.TempDir()
		child := filepath.Join(parent, "child")
		os.MkdirAll(child, 0755)

		os.WriteFile(filepath.Join(parent, "snag.toml"), []byte(`
[block]
diff = ["PARENT"]
branch = ["main"]
`), 0644)
		os.WriteFile(filepath.Join(child, "snag.toml"), []byte(`
[block]
diff = ["CHILD"]
branch = ["release/*"]
`), 0644)

		bc, found, err := walkConfig(child)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected found=true")
		}
		// Both patterns should be merged
		if len(bc.Diff) != 2 {
			t.Errorf("diff: got %v, want 2 patterns", bc.Diff)
		}
		if len(bc.Branch) != 2 {
			t.Errorf("branch: got %v, want 2 patterns", bc.Branch)
		}
	})

	t.Run("snag-local.toml merges with snag.toml", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["TEAM-PATTERN"]
branch = ["main"]
`), 0644)
		os.WriteFile(filepath.Join(dir, "snag-local.toml"), []byte(`
[block]
diff = ["PERSONAL-SECRET"]
`), 0644)

		bc, found, err := walkConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected found=true")
		}
		if len(bc.Diff) != 2 {
			t.Errorf("diff: got %v, want 2 patterns (team + personal)", bc.Diff)
		}
	})

	t.Run("snag-local.toml alone triggers TOML mode", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag-local.toml"), []byte(`
[block]
diff = ["LOCAL-ONLY"]
`), 0644)

		bc, found, err := walkConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected found=true")
		}
		if len(bc.Diff) != 1 || bc.Diff[0] != "LOCAL-ONLY" {
			t.Errorf("diff: got %v, want [LOCAL-ONLY]", bc.Diff)
		}
	})

	t.Run("no config anywhere", func(t *testing.T) {
		dir := t.TempDir()
		bc, found, err := walkConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected found=false")
		}
		if bc.HasAnyPatterns() {
			t.Errorf("expected no patterns, got %+v", bc)
		}
	})

	t.Run("push explicit empty in TOML", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
push = []
`), 0644)
		bc, _, err := walkConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Push explicitly set to empty — not nil
		if bc.Push == nil {
			t.Fatal("expected push to be non-nil (explicitly set to empty)")
		}
		if len(bc.Push) != 0 {
			t.Errorf("push: got %v, want empty", bc.Push)
		}
	})

	t.Run("push absent in TOML is nil", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
`), 0644)
		bc, _, err := walkConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bc.Push != nil {
			t.Errorf("expected push=nil, got %v", bc.Push)
		}
	})
}

func TestResolveBlockConfig(t *testing.T) {
	makeCmd := func() *cobra.Command {
		return &cobra.Command{}
	}

	t.Run("pure TOML config", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
msg  = ["WIP"]
branch = ["main"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		bc, err := resolveBlockConfig(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bc.Diff) != 1 || bc.Diff[0] != "hack" {
			t.Errorf("diff: got %v, want [hack] (lowercased)", bc.Diff)
		}
		if len(bc.Msg) != 1 || bc.Msg[0] != "wip" {
			t.Errorf("msg: got %v, want [wip]", bc.Msg)
		}
		if len(bc.Branch) != 1 || bc.Branch[0] != "main" {
			t.Errorf("branch: got %v, want [main]", bc.Branch)
		}
	})

	t.Run("branch defaults when empty", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		bc, err := resolveBlockConfig(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bc.Branch) != 2 {
			t.Errorf("branch: got %v, want [main, master]", bc.Branch)
		}
	})

	t.Run("SNAG_PROTECTED_BRANCHES overlay", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
branch = ["main"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		t.Setenv("SNAG_PROTECTED_BRANCHES", "develop, staging")

		bc, err := resolveBlockConfig(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Branch should have main + develop + staging
		if len(bc.Branch) != 3 {
			t.Errorf("branch: got %v, want 3 patterns", bc.Branch)
		}
	})

	t.Run("push fallback to diff+msg union", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
msg  = ["WIP"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		bc, err := resolveBlockConfig(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		push := bc.PushPatterns()
		if len(push) != 2 {
			t.Errorf("push patterns: got %v, want 2 (diff+msg union)", push)
		}
	})

	t.Run("push explicit overrides fallback", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
msg  = ["WIP"]
push = ["SECRET"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		bc, err := resolveBlockConfig(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		push := bc.PushPatterns()
		if len(push) != 1 || push[0] != "secret" {
			t.Errorf("push patterns: got %v, want [secret]", push)
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK", "hack", "HACK"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		bc, err := resolveBlockConfig(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(bc.Diff) != 1 {
			t.Errorf("diff: got %v, want 1 (deduplicated)", bc.Diff)
		}
	})

	t.Run("case handling: diff lowercased, branch preserved", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["UPPERCASE"]
branch = ["Release-V1"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		bc, err := resolveBlockConfig(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bc.Diff[0] != "uppercase" {
			t.Errorf("diff should be lowercased: got %q", bc.Diff[0])
		}
		if bc.Branch[0] != "Release-V1" {
			t.Errorf("branch should preserve case: got %q", bc.Branch[0])
		}
	})
}

func TestHasAnyPatterns(t *testing.T) {
	t.Run("all empty", func(t *testing.T) {
		bc := &BlockConfig{}
		if bc.HasAnyPatterns() {
			t.Error("expected false for empty config")
		}
	})

	t.Run("diff only", func(t *testing.T) {
		bc := &BlockConfig{Diff: []string{"a"}}
		if !bc.HasAnyPatterns() {
			t.Error("expected true with diff patterns")
		}
	})

	t.Run("branch only", func(t *testing.T) {
		bc := &BlockConfig{Branch: []string{"main"}}
		if !bc.HasAnyPatterns() {
			t.Error("expected true with branch patterns")
		}
	})
}

func TestPushPatterns(t *testing.T) {
	t.Run("nil push falls back to diff+msg", func(t *testing.T) {
		bc := &BlockConfig{
			Diff: []string{"a", "b"},
			Msg:  []string{"b", "c"},
		}
		push := bc.PushPatterns()
		if len(push) != 3 {
			t.Errorf("got %d patterns, want 3 (a, b, c)", len(push))
		}
	})

	t.Run("explicit push used", func(t *testing.T) {
		bc := &BlockConfig{
			Diff: []string{"a"},
			Msg:  []string{"b"},
			Push: []string{"x"},
		}
		push := bc.PushPatterns()
		if len(push) != 1 || push[0] != "x" {
			t.Errorf("got %v, want [x]", push)
		}
	})

	t.Run("explicit empty push", func(t *testing.T) {
		bc := &BlockConfig{
			Diff: []string{"a"},
			Push: []string{},
		}
		push := bc.PushPatterns()
		if len(push) != 0 {
			t.Errorf("got %v, want empty", push)
		}
	})
}
