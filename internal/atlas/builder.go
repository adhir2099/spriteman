package atlas

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adhir2099/spriteman/internal/exporter"
	"github.com/adhir2099/spriteman/internal/imaging"
	"github.com/adhir2099/spriteman/internal/packer"
)

const Version = "1.0.0"

// Config defines all parameters for building a sprite atlas.
type Config struct {
	InputDir   string            `json:"input"`
	OutputPNG  string            `json:"output"`
	OutputMeta string            `json:"meta"`
	Format     exporter.Format   `json:"format"`
	Algorithm  packer.Algorithm  `json:"algorithm"`
	MaxWidth   int               `json:"maxWidth"`
	MaxHeight  int               `json:"maxHeight"`
	Padding    int               `json:"padding"`
	Extrude    int               `json:"extrude"`
	Trim       bool              `json:"trim"`
	Dedupe     bool              `json:"dedupe"`
	PowerOfTwo bool              `json:"powerOfTwo"`
	Square     bool              `json:"square"`
	Optimize   bool              `json:"optimize"`
	Recursive  bool              `json:"recursive"`
}

// DefaultConfig returns sensible production defaults for 2D game engines.
func DefaultConfig() Config {
	return Config{
		InputDir:   ".",
		OutputPNG:  "spritesheet.png",
		OutputMeta: "",
		Format:     exporter.FormatJSONHash,
		Algorithm:  packer.AlgorithmMaxRects,
		MaxWidth:   4096,
		MaxHeight:  4096,
		Padding:    2,
		Extrude:    0,
		Trim:       true,
		Dedupe:     true,
		PowerOfTwo: false,
		Square:     false,
		Optimize:   false,
		Recursive:  true,
	}
}

// BuildReport summarizes the results and performance metrics of an atlas build.
type BuildReport struct {
	OutputPNG        string              `json:"outputPng"`
	OutputMeta       string              `json:"outputMeta"`
	Format           exporter.Format     `json:"format"`
	Algorithm        packer.Algorithm    `json:"algorithm"`
	AtlasWidth       int                 `json:"atlasWidth"`
	AtlasHeight      int                 `json:"atlasHeight"`
	TotalSprites     int                 `json:"totalSprites"`
	UniqueSprites    int                 `json:"uniqueSprites"`
	Deduplicated     int                 `json:"deduplicated"`
	TrimmedCount     int                 `json:"trimmedCount"`
	Efficiency       float64             `json:"efficiency"`
	InputBytes       int64               `json:"inputBytes"`
	OutputBytes      int64               `json:"outputBytes"`
	Duration         time.Duration       `json:"durationNs"`
	DurationMS       float64             `json:"durationMs"`
	Document         *exporter.AtlasDocument `json:"-"`
}

// Build scans InputDir, loads and trims PNGs, packs them into a single texture atlas, and exports metadata.
func Build(cfg Config) (*BuildReport, error) {
	start := time.Now()

	if cfg.Format == "" {
		cfg.Format = exporter.FormatJSONHash
	}
	if cfg.Algorithm == "" {
		cfg.Algorithm = packer.AlgorithmMaxRects
	}
	if cfg.OutputPNG == "" {
		cfg.OutputPNG = "spritesheet.png"
	}
	if cfg.OutputMeta == "" {
		ext := exporter.DefaultExtension(cfg.Format)
		base := strings.TrimSuffix(cfg.OutputPNG, filepath.Ext(cfg.OutputPNG))
		cfg.OutputMeta = base + ext
	}

	files, err := discoverPNGs(cfg.InputDir, cfg.OutputPNG, cfg.Recursive)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .png files found in %q", cfg.InputDir)
	}

	sprites, err := loadSpritesParallel(cfg.InputDir, files, cfg.Trim)
	if err != nil {
		return nil, err
	}

	// Sort sprites deterministically by ID
	sort.Slice(sprites, func(i, j int) bool {
		return sprites[i].ID < sprites[j].ID
	})

	// Deduplicate identical sprites if requested
	var uniqueSprites []*imaging.SpriteData
	hashToPrimaryID := make(map[string]string, len(sprites))
	aliasToPrimaryID := make(map[string]string)
	var totalInputBytes int64
	trimmedCount := 0

	for _, sp := range sprites {
		totalInputBytes += sp.FileSize
		if sp.WasTrimmed {
			trimmedCount++
		}
		if cfg.Dedupe {
			if primaryID, exists := hashToPrimaryID[sp.Hash]; exists {
				aliasToPrimaryID[sp.ID] = primaryID
				continue
			}
			hashToPrimaryID[sp.Hash] = sp.ID
		}
		uniqueSprites = append(uniqueSprites, sp)
	}

	packItems := make([]packer.Item, len(uniqueSprites))
	spriteByID := make(map[string]*imaging.SpriteData, len(uniqueSprites))
	for i, sp := range uniqueSprites {
		packItems[i] = packer.Item{
			ID:     sp.ID,
			Width:  sp.TrimmedW,
			Height: sp.TrimmedH,
		}
		spriteByID[sp.ID] = sp
	}

	packRes, err := packer.Pack(packItems, packer.Options{
		Algorithm:  cfg.Algorithm,
		MaxWidth:   cfg.MaxWidth,
		MaxHeight:  cfg.MaxHeight,
		Padding:    cfg.Padding,
		Extrude:    cfg.Extrude,
		PowerOfTwo: cfg.PowerOfTwo,
		Square:     cfg.Square,
	})
	if err != nil {
		return nil, err
	}

	// Composite all unique sprites onto the final RGBA atlas canvas
	atlasImg := image.NewNRGBA(image.Rect(0, 0, packRes.Width, packRes.Height))
	for _, sp := range uniqueSprites {
		rect := packRes.Placements[sp.ID]
		imaging.DrawSpriteWithExtrude(atlasImg, sp.Image, rect, cfg.Extrude)
	}

	// Ensure parent directories exist
	if dir := filepath.Dir(cfg.OutputPNG); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}
	}
	if dir := filepath.Dir(cfg.OutputMeta); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create meta dir: %w", err)
		}
	}

	outBytes, err := imaging.SavePNG(cfg.OutputPNG, atlasImg, cfg.Optimize)
	if err != nil {
		return nil, err
	}

	// Build frame entries (including deduplicated aliases)
	frames := make([]exporter.FrameEntry, 0, len(sprites))
	for _, sp := range sprites {
		lookupID := sp.ID
		aliasOf := ""
		if primaryID, isAlias := aliasToPrimaryID[sp.ID]; isAlias {
			lookupID = primaryID
			aliasOf = primaryID
		}
		rect := packRes.Placements[lookupID]
		frames = append(frames, exporter.FrameEntry{
			Name:    sp.ID,
			Frame:   rect,
			Rotated: false,
			Trimmed: sp.WasTrimmed,
			SpriteSourceSize: packer.Rect{
				X:      sp.TrimX,
				Y:      sp.TrimY,
				Width:  sp.TrimmedW,
				Height: sp.TrimmedH,
			},
			SourceSize: exporter.Size2D{
				W: sp.SourceW,
				H: sp.SourceH,
			},
			Pivot:   exporter.Pivot2D{X: 0.5, Y: 0.5},
			AliasOf: aliasOf,
		})
	}

	pixelFmt := "RGBA8888"
	if cfg.Optimize {
		pixelFmt = "INDEXED8"
	}

	doc := &exporter.AtlasDocument{
		Frames: frames,
		Meta: exporter.MetaInfo{
			App:        "https://github.com/adhir2099/spriteman",
			Version:    Version,
			Image:      filepath.Base(cfg.OutputPNG),
			Format:     pixelFmt,
			Size:       exporter.Size2D{W: packRes.Width, H: packRes.Height},
			Scale:      "1",
			Algorithm:  string(cfg.Algorithm),
			Efficiency: packRes.Efficiency,
		},
	}

	if err := exporter.Export(doc, cfg.Format, cfg.OutputMeta); err != nil {
		return nil, fmt.Errorf("export metadata: %w", err)
	}

	dur := time.Since(start)
	return &BuildReport{
		OutputPNG:     cfg.OutputPNG,
		OutputMeta:    cfg.OutputMeta,
		Format:        cfg.Format,
		Algorithm:     cfg.Algorithm,
		AtlasWidth:    packRes.Width,
		AtlasHeight:   packRes.Height,
		TotalSprites:  len(sprites),
		UniqueSprites: len(uniqueSprites),
		Deduplicated:  len(sprites) - len(uniqueSprites),
		TrimmedCount:  trimmedCount,
		Efficiency:    packRes.Efficiency,
		InputBytes:    totalInputBytes,
		OutputBytes:   outBytes,
		Duration:      dur,
		DurationMS:    float64(dur.Microseconds()) / 1000.0,
		Document:      doc,
	}, nil
}

// Slice extracts individual sprites from a packed sprite sheet PNG using its JSON metadata,
// restoring original un-trimmed dimensions.
func Slice(sheetPath, metaPath, outDir string) (int, error) {
	doc, err := exporter.ParseJSONFile(metaPath)
	if err != nil {
		return 0, err
	}

	f, err := os.Open(sheetPath)
	if err != nil {
		return 0, fmt.Errorf("open sheet %s: %w", sheetPath, err)
	}
	defer f.Close()

	rawSheet, err := png.Decode(f)
	if err != nil {
		return 0, fmt.Errorf("decode sheet %s: %w", sheetPath, err)
	}
	sheet := imaging.ToNRGBA(rawSheet)

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return 0, fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	for _, fr := range doc.Frames {
		outW := fr.SourceSize.W
		outH := fr.SourceSize.H
		if outW <= 0 {
			outW = fr.Frame.Width
		}
		if outH <= 0 {
			outH = fr.Frame.Height
		}

		canvas := image.NewNRGBA(image.Rect(0, 0, outW, outH))
		dstX := fr.SpriteSourceSize.X
		dstY := fr.SpriteSourceSize.Y
		dstRect := image.Rect(dstX, dstY, dstX+fr.Frame.Width, dstY+fr.Frame.Height)
		srcPt := image.Point{X: fr.Frame.X, Y: fr.Frame.Y}
		draw.Draw(canvas, dstRect, sheet, srcPt, draw.Src)

		cleanName := strings.TrimSuffix(fr.Name, ".png") + ".png"
		targetPath := filepath.Join(outDir, filepath.FromSlash(cleanName))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return 0, err
		}
		if _, err := imaging.SavePNG(targetPath, canvas, false); err != nil {
			return 0, err
		}
	}

	return len(doc.Frames), nil
}

// CrushReport summarizes a batch PNG compression run.
type CrushReport struct {
	FilesProcessed int
	BeforeBytes    int64
	AfterBytes     int64
	Duration       time.Duration
}

// Crush optimizes a single PNG file or all PNGs in a directory in-place.
func Crush(target string, quantize bool) (*CrushReport, error) {
	start := time.Now()
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	var files []string
	if !info.IsDir() {
		files = []string{target}
	} else {
		files, err = discoverPNGs(target, "", true)
		if err != nil {
			return nil, err
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .png files found in %q", target)
	}

	var beforeTotal, afterTotal int64
	for _, p := range files {
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		beforeTotal += st.Size()

		sp, err := imaging.LoadSprite(p, filepath.Base(p), false)
		if err != nil {
			return nil, err
		}

		written, err := imaging.SavePNG(p, sp.Image, quantize)
		if err != nil {
			return nil, err
		}
		afterTotal += written
	}

	return &CrushReport{
		FilesProcessed: len(files),
		BeforeBytes:    beforeTotal,
		AfterBytes:     afterTotal,
		Duration:       time.Since(start),
	}, nil
}

func discoverPNGs(inputDir, excludePath string, recursive bool) ([]string, error) {
	absExclude := ""
	if excludePath != "" {
		if abs, err := filepath.Abs(excludePath); err == nil {
			absExclude = strings.ToLower(abs)
		}
	}

	var files []string
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !recursive && path != inputDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".png") {
			if abs, err := filepath.Abs(path); err == nil && strings.ToLower(abs) == absExclude {
				return nil
			}
			files = append(files, path)
		}
		return nil
	}

	if err := filepath.WalkDir(inputDir, walkFn); err != nil {
		return nil, fmt.Errorf("scan directory %s: %w", inputDir, err)
	}
	return files, nil
}

func loadSpritesParallel(rootDir string, files []string, trimAlpha bool) ([]*imaging.SpriteData, error) {
	workers := min(runtime.NumCPU(), len(files))
	if workers < 1 {
		workers = 1
	}

	type result struct {
		sprite *imaging.SpriteData
		err    error
	}

	jobs := make(chan string, len(files))
	results := make(chan result, len(files))

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				rel, err := filepath.Rel(rootDir, path)
				if err != nil {
					rel = filepath.Base(path)
				}
				id := strings.TrimSuffix(filepath.ToSlash(rel), ".png")
				sp, err := imaging.LoadSprite(path, id, trimAlpha)
				results <- result{sprite: sp, err: err}
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	close(results)

	sprites := make([]*imaging.SpriteData, 0, len(files))
	for r := range results {
		if r.err != nil {
			return nil, r.err
		}
		sprites = append(sprites, r.sprite)
	}
	return sprites, nil
}
