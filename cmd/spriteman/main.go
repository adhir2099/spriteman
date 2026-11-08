package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/adhir2099/spriteman/internal/atlas"
	"github.com/adhir2099/spriteman/internal/exporter"
	"github.com/adhir2099/spriteman/internal/packer"
)

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cCyan   = "\033[36m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cPurple = "\033[35m"
	cGray   = "\033[90m"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "\n  error: %v\n\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printBanner()
		printUsage()
		return nil
	}

	sub := args[0]
	switch sub {
	case "-h", "--help", "help":
		printBanner()
		printUsage()
		return nil
	case "-v", "--version", "version":
		fmt.Printf("spriteman v%s\n", atlas.Version)
		return nil
	case "pack":
		return runPack(args[1:])
	case "slice":
		return runSlice(args[1:])
	case "crush":
		return runCrush(args[1:])
	case "inspect":
		return runInspect(args[1:])
	case "init":
		return runInit(args[1:])
	case "demo":
		return runDemo(args[1:])
	default:
		// Allow running `spriteman ./sprites -o atlas.png` directly without typing `pack`
		return runPack(args)
	}
}

func runPack(args []string) error {
	cfg := atlas.DefaultConfig()
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)

	var (
		configPath string
		formatStr  string
		algStr     string
		jsonOut    bool
		preview    bool
		noColor    bool
	)

	fs.StringVar(&configPath, "config", "", "Path to spriteman.json config file")
	fs.StringVar(&cfg.InputDir, "i", cfg.InputDir, "Input directory containing PNG sprites")
	fs.StringVar(&cfg.InputDir, "input", cfg.InputDir, "Input directory containing PNG sprites")
	fs.StringVar(&cfg.OutputPNG, "o", cfg.OutputPNG, "Output sprite sheet PNG path")
	fs.StringVar(&cfg.OutputPNG, "output", cfg.OutputPNG, "Output sprite sheet PNG path")
	fs.StringVar(&cfg.OutputMeta, "meta", "", "Output metadata file path (defaults to <output>.<ext>)")
	fs.StringVar(&formatStr, "f", string(cfg.Format), "Export format: json-hash, json-array, godot, css")
	fs.StringVar(&formatStr, "format", string(cfg.Format), "Export format: json-hash, json-array, godot, css")
	fs.StringVar(&algStr, "alg", string(cfg.Algorithm), "Packing algorithm: maxrects, shelf, grid")
	fs.IntVar(&cfg.MaxWidth, "max-width", cfg.MaxWidth, "Maximum atlas width in pixels")
	fs.IntVar(&cfg.MaxHeight, "max-height", cfg.MaxHeight, "Maximum atlas height in pixels")
	fs.IntVar(&cfg.Padding, "padding", cfg.Padding, "Transparent pixel padding between sprites")
	fs.IntVar(&cfg.Extrude, "extrude", cfg.Extrude, "Edge pixel extrusion count to prevent WebGL mipmap bleeding")
	fs.BoolVar(&cfg.Trim, "trim", cfg.Trim, "Trim transparent borders from sprites")
	fs.BoolVar(&cfg.Dedupe, "dedupe", cfg.Dedupe, "Deduplicate identical sprite frames")
	fs.BoolVar(&cfg.PowerOfTwo, "pot", cfg.PowerOfTwo, "Force power-of-two atlas dimensions (256, 512, 1024...)")
	fs.BoolVar(&cfg.Square, "square", cfg.Square, "Force square atlas dimensions (width == height)")
	fs.BoolVar(&cfg.Optimize, "optimize", cfg.Optimize, "Quantize to 8-bit 256-color indexed PNG for smaller file size")
	fs.BoolVar(&cfg.Recursive, "recursive", cfg.Recursive, "Scan subdirectories recursively")
	fs.BoolVar(&preview, "preview", true, "Render ASCII layout preview in terminal")
	fs.BoolVar(&jsonOut, "json", false, "Print machine-readable JSON build report")
	fs.BoolVar(&noColor, "no-color", false, "Disable ANSI color output")

	// Extract positional directory argument even if placed before flags
	var flagArgs []string
	var positionalDir string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") && positionalDir == "" {
			positionalDir = arg
			continue
		}
		flagArgs = append(flagArgs, arg)
		// If this flag takes a value and isn't using =, keep next token with flagArgs
		if strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") && isValueFlag(arg) && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if configPath != "" {
		raw, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read config %s: %w", configPath, err)
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("parse config %s: %w", configPath, err)
		}
		// Re-parse flags so explicit CLI flags override config file values
		_ = fs.Parse(flagArgs)
	}

	if positionalDir != "" {
		cfg.InputDir = positionalDir
	} else if fs.NArg() > 0 {
		cfg.InputDir = fs.Arg(0)
	}

	cfg.Format = exporter.Format(strings.ToLower(formatStr))
	cfg.Algorithm = packer.Algorithm(strings.ToLower(algStr))

	report, err := atlas.Build(cfg)
	if err != nil {
		return err
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printBuildReport(report, preview, noColor)
	return nil
}

func isValueFlag(arg string) bool {
	name := strings.TrimLeft(arg, "-")
	switch name {
	case "config", "i", "input", "o", "output", "meta", "f", "format", "alg", "max-width", "max-height", "padding", "extrude":
		return true
	default:
		return false
	}
}

func runSlice(args []string) error {
	fs := flag.NewFlagSet("slice", flag.ContinueOnError)
	var metaPath, outDir string
	fs.StringVar(&metaPath, "meta", "", "Path to atlas JSON metadata file")
	fs.StringVar(&outDir, "o", "sliced_sprites", "Output directory for extracted frames")
	fs.StringVar(&outDir, "output", "sliced_sprites", "Output directory for extracted frames")

	var flagArgs []string
	var sheetPath string
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") && sheetPath == "" {
			sheetPath = args[i]
			continue
		}
		flagArgs = append(flagArgs, args[i])
		if (args[i] == "-meta" || args[i] == "--meta" || args[i] == "-o" || args[i] == "--output") && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if sheetPath == "" && fs.NArg() > 0 {
		sheetPath = fs.Arg(0)
	}
	if sheetPath == "" {
		return fmt.Errorf("usage: spriteman slice <spritesheet.png> --meta <spritesheet.json> -o <output-dir>")
	}
	if metaPath == "" {
		metaPath = strings.TrimSuffix(sheetPath, filepath.Ext(sheetPath)) + ".json"
	}

	count, err := atlas.Slice(sheetPath, metaPath, outDir)
	if err != nil {
		return err
	}
	fmt.Printf("\n  %s✓ Sliced %d sprites%s from %s into %s\n\n", cGreen, count, cReset, sheetPath, outDir)
	return nil
}

func runCrush(args []string) error {
	fs := flag.NewFlagSet("crush", flag.ContinueOnError)
	var quantize bool
	fs.BoolVar(&quantize, "quantize", true, "Quantize to 256-color indexed palette")

	var flagArgs []string
	var target string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && target == "" {
			target = a
			continue
		}
		flagArgs = append(flagArgs, a)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if target == "" {
		target = "."
	}

	rep, err := atlas.Crush(target, quantize)
	if err != nil {
		return err
	}
	savedPct := 0.0
	if rep.BeforeBytes > 0 {
		savedPct = (1.0 - float64(rep.AfterBytes)/float64(rep.BeforeBytes)) * 100.0
	}
	fmt.Printf("\n  %s✓ Crushed %d PNG file(s)%s in %.1fms\n", cGreen, rep.FilesProcessed, cReset, float64(rep.Duration.Microseconds())/1000.0)
	fmt.Printf("    Before: %s  ->  After: %s  (%.1f%% reduction)\n\n",
		formatBytes(rep.BeforeBytes), formatBytes(rep.AfterBytes), savedPct)
	return nil
}

func runInspect(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: spriteman inspect <spritesheet.json>")
	}
	doc, err := exporter.ParseJSONFile(args[0])
	if err != nil {
		return err
	}

	trimmedCount := 0
	aliasCount := 0
	usedArea := 0
	seenPrimary := make(map[string]bool)

	for _, f := range doc.Frames {
		if f.Trimmed {
			trimmedCount++
		}
		if f.AliasOf != "" {
			aliasCount++
		} else {
			key := fmt.Sprintf("%d,%d,%d,%d", f.Frame.X, f.Frame.Y, f.Frame.Width, f.Frame.Height)
			if !seenPrimary[key] {
				seenPrimary[key] = true
				usedArea += f.Frame.Width * f.Frame.Height
			}
		}
	}

	totalArea := doc.Meta.Size.W * doc.Meta.Size.H
	eff := 0.0
	if totalArea > 0 {
		eff = (float64(usedArea) / float64(totalArea)) * 100.0
	}

	fmt.Printf("\n  %s%sAtlas Inspection:%s %s\n", cBold, cCyan, cReset, args[0])
	fmt.Printf("  ──────────────────────────────────────────\n")
	fmt.Printf("  Image:         %s (%dx%d, %s)\n", doc.Meta.Image, doc.Meta.Size.W, doc.Meta.Size.H, doc.Meta.Format)
	fmt.Printf("  Total Frames:  %d (%d unique, %d deduplicated aliases)\n", len(doc.Frames), len(doc.Frames)-aliasCount, aliasCount)
	fmt.Printf("  Alpha Trimmed: %d frame(s)\n", trimmedCount)
	fmt.Printf("  Occupancy:     %.1f%% (%d / %d px²)\n\n", eff, usedArea, totalArea)
	return nil
}

func runInit(args []string) error {
	outPath := "spriteman.json"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		outPath = args[0]
	}
	cfg := atlas.DefaultConfig()
	cfg.InputDir = "./sprites"
	cfg.OutputPNG = "./dist/spritesheet.png"
	cfg.OutputMeta = "./dist/spritesheet.json"
	cfg.Optimize = true

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("\n  %s✓ Created %s%s\n  Run `spriteman pack --config %s` to build your atlas.\n\n", cGreen, outPath, cReset, outPath)
	return nil
}

func runDemo(args []string) error {
	outDir := filepath.Join("examples", "demo-sprites")
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		outDir = args[0]
	}
	if err := generateDemoSprites(outDir); err != nil {
		return err
	}
	fmt.Printf("\n  %s✓ Generated 10 RPG pixel-art sprites in %s%s\n", cGreen, outDir, cReset)

	distDir := filepath.Join(filepath.Dir(outDir), "dist")
	return runPack([]string{
		outDir,
		"-o", filepath.Join(distDir, "rpg_atlas.png"),
		"--format", "json-hash",
		"--padding", "2",
		"--extrude", "1",
		"--optimize",
	})
}

func printBuildReport(r *atlas.BuildReport, showPreview, noColor bool) {
	col := func(c string) string {
		if noColor {
			return ""
		}
		return c
	}

	fmt.Printf("\n  %s%s⚡ spriteman v%s%s — 2D Sprite Sheet & Asset Packer\n", col(cBold), col(cCyan), atlas.Version, col(cReset))
	fmt.Printf("  %s──────────────────────────────────────────────────────%s\n", col(cGray), col(cReset))
	fmt.Printf("  %sAtlas Output:%s  %s (%dx%d px)\n", col(cBold), col(cReset), r.OutputPNG, r.AtlasWidth, r.AtlasHeight)
	fmt.Printf("  %sMetadata:%s      %s (%s)\n", col(cBold), col(cReset), r.OutputMeta, r.Format)
	fmt.Printf("  %sAlgorithm:%s     %s\n", col(cBold), col(cReset), r.Algorithm)
	fmt.Printf("  %sSprites:%s       %d total (%d packed, %d deduplicated, %d alpha-trimmed)\n",
		col(cBold), col(cReset), r.TotalSprites, r.UniqueSprites, r.Deduplicated, r.TrimmedCount)
	fmt.Printf("  %sEfficiency:%s    %s%.1f%%%s density\n",
		col(cBold), col(cReset), col(cGreen), r.Efficiency, col(cReset))
	fmt.Printf("  %sBundle Size:%s   %s -> %s%s%s (completed in %s%.2fms%s)\n",
		col(cBold), col(cReset),
		formatBytes(r.InputBytes),
		col(cYellow), formatBytes(r.OutputBytes), col(cReset),
		col(cPurple), r.DurationMS, col(cReset))

	if showPreview && r.Document != nil {
		fmt.Println()
		renderASCIIPreview(r.Document, r.AtlasWidth, r.AtlasHeight, noColor)
	}
	fmt.Println()
}

func renderASCIIPreview(doc *exporter.AtlasDocument, atlasW, atlasH int, noColor bool) {
	const gridW = 48
	const gridH = 16

	grid := make([][]byte, gridH)
	for y := 0; y < gridH; y++ {
		grid[y] = make([]byte, gridW)
		for x := 0; x < gridW; x++ {
			grid[y][x] = '.'
		}
	}

	symbols := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789#@%&*"
	symIdx := 0
	for _, f := range doc.Frames {
		if f.AliasOf != "" {
			continue
		}
		sym := symbols[symIdx%len(symbols)]
		symIdx++

		x0 := (f.Frame.X * gridW) / max(1, atlasW)
		y0 := (f.Frame.Y * gridH) / max(1, atlasH)
		x1 := ((f.Frame.X + f.Frame.Width) * gridW) / max(1, atlasW)
		y1 := ((f.Frame.Y + f.Frame.Height) * gridH) / max(1, atlasH)
		if x1 <= x0 {
			x1 = min(gridW, x0+1)
		}
		if y1 <= y0 {
			y1 = min(gridH, y0+1)
		}

		for y := max(0, y0); y < min(gridH, y1); y++ {
			for x := max(0, x0); x < min(gridW, x1); x++ {
				grid[y][x] = sym
			}
		}
	}

	borderCol := cGray
	fillCol := cCyan
	reset := cReset
	if noColor {
		borderCol, fillCol, reset = "", "", ""
	}

	fmt.Printf("  %s┌%s┐%s\n", borderCol, strings.Repeat("─", gridW), reset)
	for y := 0; y < gridH; y++ {
		fmt.Printf("  %s│%s", borderCol, reset)
		for x := 0; x < gridW; x++ {
			ch := grid[y][x]
			if ch == '.' {
				fmt.Printf("%s·%s", borderCol, reset)
			} else {
				fmt.Printf("%s%c%s", fillCol, ch, reset)
			}
		}
		fmt.Printf("%s│%s\n", borderCol, reset)
	}
	fmt.Printf("  %s└%s┘%s\n", borderCol, strings.Repeat("─", gridW), reset)
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	kb := float64(b) / 1024.0
	if kb < 1024 {
		return fmt.Sprintf("%.1f KB", kb)
	}
	return fmt.Sprintf("%.2f MB", kb/1024.0)
}

func printBanner() {
	fmt.Printf(`
  %s%s⚡ spriteman v%s%s
  Fast, zero-dependency 2D Sprite Sheet & Asset Packer CLI in Go.
`, cBold, cCyan, atlas.Version, cReset)
}

func printUsage() {
	fmt.Printf(`
  %sUSAGE:%s
    spriteman pack <input-dir> [flags]
    spriteman slice <sheet.png> --meta <sheet.json> -o <out-dir>
    spriteman crush <dir-or-file> [--quantize]
    spriteman inspect <sheet.json>
    spriteman init [spriteman.json]
    spriteman demo [examples/demo-sprites]

  %sPACK FLAGS:%s
    -i, --input <dir>       Input folder containing PNG sprites (default ".")
    -o, --output <file>     Output sprite sheet PNG path (default "spritesheet.png")
    --meta <file>           Output metadata path (defaults to <output>.<ext>)
    -f, --format <fmt>      Export format: json-hash, json-array, godot, css (default "json-hash")
    --alg <algorithm>       Packing algorithm: maxrects, shelf, grid (default "maxrects")
    --padding <px>          Transparent spacing between sprites (default 2)
    --extrude <px>          Edge pixel extrusion count to prevent WebGL bleed (default 0)
    --trim                  Strip transparent margins & store original offsets (default true)
    --dedupe                Alias identical frames to the same atlas coordinates (default true)
    --pot                   Force power-of-two texture dimensions (256, 512, 1024...)
    --square                Force square texture dimensions (width == height)
    --optimize              Quantize to 8-bit 256-color indexed PNG for 50-70%% smaller files
    --max-width <px>        Maximum atlas width (default 4096)
    --max-height <px>       Maximum atlas height (default 4096)
    --config <file>         Load options from a spriteman.json configuration file
    --json                  Output machine-readable JSON build report

  %sEXAMPLES:%s
    spriteman pack ./assets/sprites -o ./dist/characters.png --pot --optimize
    spriteman pack ./ui-icons -o ./public/icons.png --format css
    spriteman pack ./hero-frames -o ./godot/hero.png --format godot --extrude 1
    spriteman demo
`, cBold, cReset, cBold, cReset, cBold, cReset)
}

func generateDemoSprites(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	type spec struct {
		name    string
		canvasW int
		canvasH int
		drawFn  func(img *image.NRGBA)
	}

	drawCircle := func(img *image.NRGBA, cx, cy, r int, fill, border color.NRGBA) {
		for y := cy - r; y <= cy+r; y++ {
			for x := cx - r; x <= cx+r; x++ {
				dx, dy := x-cx, y-cy
				d2 := dx*dx + dy*dy
				if d2 <= r*r {
					if d2 >= (r-2)*(r-2) {
						img.SetNRGBA(x, y, border)
					} else {
						img.SetNRGBA(x, y, fill)
					}
				}
			}
		}
	}

	drawRect := func(img *image.NRGBA, x0, y0, w, h int, fill, border color.NRGBA) {
		for y := y0; y < y0+h; y++ {
			for x := x0; x < x0+w; x++ {
				if x == x0 || x == x0+w-1 || y == y0 || y == y0+h-1 {
					img.SetNRGBA(x, y, border)
				} else {
					img.SetNRGBA(x, y, fill)
				}
			}
		}
	}

	specs := []spec{
		{
			name: "hero_idle_1.png", canvasW: 64, canvasH: 64,
			drawFn: func(img *image.NRGBA) {
				// Padded 64x64 canvas with a 36x48 knight in the center (demonstrates alpha trimming!)
				drawRect(img, 14, 8, 36, 48, color.NRGBA{59, 130, 246, 255}, color.NRGBA{29, 78, 216, 255})
				drawRect(img, 20, 14, 24, 14, color.NRGBA{253, 224, 71, 255}, color.NRGBA{161, 98, 7, 255})
			},
		},
		{
			// Identical to hero_idle_1 to demonstrate automatic frame deduplication!
			name: "hero_idle_2_duplicate.png", canvasW: 64, canvasH: 64,
			drawFn: func(img *image.NRGBA) {
				drawRect(img, 14, 8, 36, 48, color.NRGBA{59, 130, 246, 255}, color.NRGBA{29, 78, 216, 255})
				drawRect(img, 20, 14, 24, 14, color.NRGBA{253, 224, 71, 255}, color.NRGBA{161, 98, 7, 255})
			},
		},
		{
			name: "hero_attack.png", canvasW: 64, canvasH: 64,
			drawFn: func(img *image.NRGBA) {
				drawRect(img, 10, 8, 44, 48, color.NRGBA{37, 99, 235, 255}, color.NRGBA{30, 58, 138, 255})
				drawRect(img, 42, 20, 16, 8, color.NRGBA{226, 232, 240, 255}, color.NRGBA{71, 85, 105, 255})
			},
		},
		{
			name: "slime_green.png", canvasW: 48, canvasH: 48,
			drawFn: func(img *image.NRGBA) {
				drawCircle(img, 24, 28, 16, color.NRGBA{74, 222, 128, 255}, color.NRGBA{21, 128, 61, 255})
			},
		},
		{
			name: "goblin_scout.png", canvasW: 48, canvasH: 56,
			drawFn: func(img *image.NRGBA) {
				drawRect(img, 8, 6, 32, 44, color.NRGBA{132, 204, 22, 255}, color.NRGBA{63, 98, 18, 255})
			},
		},
		{
			name: "gold_coin.png", canvasW: 32, canvasH: 32,
			drawFn: func(img *image.NRGBA) {
				drawCircle(img, 16, 16, 11, color.NRGBA{250, 204, 21, 255}, color.NRGBA{180, 83, 9, 255})
			},
		},
		{
			name: "health_potion.png", canvasW: 32, canvasH: 36,
			drawFn: func(img *image.NRGBA) {
				drawCircle(img, 16, 22, 10, color.NRGBA{244, 63, 94, 255}, color.NRGBA{159, 18, 57, 255})
				drawRect(img, 13, 6, 6, 8, color.NRGBA{214, 211, 209, 255}, color.NRGBA{120, 113, 108, 255})
			},
		},
		{
			name: "iron_sword.png", canvasW: 28, canvasH: 56,
			drawFn: func(img *image.NRGBA) {
				drawRect(img, 10, 4, 8, 36, color.NRGBA{226, 232, 240, 255}, color.NRGBA{100, 116, 139, 255})
				drawRect(img, 4, 38, 20, 5, color.NRGBA{234, 179, 8, 255}, color.NRGBA{161, 98, 7, 255})
				drawRect(img, 11, 43, 6, 9, color.NRGBA{120, 53, 15, 255}, color.NRGBA{69, 26, 3, 255})
			},
		},
		{
			name: "oak_shield.png", canvasW: 40, canvasH: 44,
			drawFn: func(img *image.NRGBA) {
				drawRect(img, 6, 4, 28, 34, color.NRGBA{180, 83, 9, 255}, color.NRGBA{148, 163, 184, 255})
			},
		},
		{
			name: "treasure_chest.png", canvasW: 56, canvasH: 44,
			drawFn: func(img *image.NRGBA) {
				drawRect(img, 4, 8, 48, 30, color.NRGBA{146, 64, 14, 255}, color.NRGBA{250, 204, 21, 255})
				drawRect(img, 24, 18, 8, 8, color.NRGBA{250, 204, 21, 255}, color.NRGBA{113, 63, 18, 255})
			},
		},
	}

	for _, sp := range specs {
		img := image.NewNRGBA(image.Rect(0, 0, sp.canvasW, sp.canvasH))
		sp.drawFn(img)
		f, err := os.Create(filepath.Join(dir, sp.name))
		if err != nil {
			return err
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}
