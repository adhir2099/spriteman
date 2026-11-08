package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLISubcommands(t *testing.T) {
	tmp := t.TempDir()

	// 1. Test `init` subcommand
	cfgFile := filepath.Join(tmp, "spriteman.json")
	if err := run([]string{"init", cfgFile}); err != nil {
		t.Fatalf("run(init) failed: %v", err)
	}
	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("expected config file %s to exist: %v", cfgFile, err)
	}

	// 2. Test `demo` subcommand (generates demo sprites and packs them)
	demoDir := filepath.Join(tmp, "demo-sprites")
	if err := run([]string{"demo", demoDir}); err != nil {
		t.Fatalf("run(demo) failed: %v", err)
	}

	outPNG := filepath.Join(tmp, "dist", "rpg_atlas.png")
	outJSON := filepath.Join(tmp, "dist", "rpg_atlas.json")
	if _, err := os.Stat(outPNG); err != nil {
		t.Fatalf("expected packed atlas PNG %s: %v", outPNG, err)
	}
	if _, err := os.Stat(outJSON); err != nil {
		t.Fatalf("expected packed atlas JSON %s: %v", outJSON, err)
	}

	// 3. Test `inspect` subcommand
	if err := run([]string{"inspect", outJSON}); err != nil {
		t.Fatalf("run(inspect) failed: %v", err)
	}

	// 4. Test `slice` subcommand
	slicedDir := filepath.Join(tmp, "sliced")
	if err := run([]string{"slice", outPNG, "--meta", outJSON, "-o", slicedDir}); err != nil {
		t.Fatalf("run(slice) failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(slicedDir, "hero_idle_1.png")); err != nil {
		t.Fatalf("expected sliced frame hero_idle_1.png: %v", err)
	}

	// 5. Test `crush` subcommand
	if err := run([]string{"crush", slicedDir}); err != nil {
		t.Fatalf("run(crush) failed: %v", err)
	}
}
