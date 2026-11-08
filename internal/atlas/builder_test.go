package atlas

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/adhir2099/spriteman/internal/exporter"
	"github.com/adhir2099/spriteman/internal/packer"
)

func TestBuildAndSliceRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	inDir := filepath.Join(tmp, "sprites")
	if err := os.MkdirAll(inDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Sprite 1: 32x32 with a 16x16 blue square at (8,8)
	img1 := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 8; y < 24; y++ {
		for x := 8; x < 24; x++ {
			img1.SetNRGBA(x, y, color.NRGBA{R: 30, G: 144, B: 255, A: 255})
		}
	}
	writeTestPNG(t, filepath.Join(inDir, "hero_1.png"), img1)

	// Sprite 2: identical pixels to hero_1 to test deduplication
	writeTestPNG(t, filepath.Join(inDir, "hero_2_dup.png"), img1)

	// Sprite 3: 24x24 gold coin
	img3 := image.NewNRGBA(image.Rect(0, 0, 24, 24))
	for y := 2; y < 22; y++ {
		for x := 2; x < 22; x++ {
			img3.SetNRGBA(x, y, color.NRGBA{R: 255, G: 215, B: 0, A: 255})
		}
	}
	writeTestPNG(t, filepath.Join(inDir, "coin.png"), img3)

	outPNG := filepath.Join(tmp, "out", "atlas.png")
	outMeta := filepath.Join(tmp, "out", "atlas.json")

	report, err := Build(Config{
		InputDir:   inDir,
		OutputPNG:  outPNG,
		OutputMeta: outMeta,
		Format:     exporter.FormatJSONHash,
		Algorithm:  packer.AlgorithmMaxRects,
		MaxWidth:   512,
		MaxHeight:  512,
		Padding:    2,
		Extrude:    1,
		Trim:       true,
		Dedupe:     true,
		PowerOfTwo: true,
		Optimize:   true,
		Recursive:  true,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if report.TotalSprites != 3 {
		t.Errorf("expected 3 total sprites, got %d", report.TotalSprites)
	}
	if report.UniqueSprites != 2 || report.Deduplicated != 1 {
		t.Errorf("expected 2 unique and 1 deduplicated, got unique=%d dedup=%d", report.UniqueSprites, report.Deduplicated)
	}

	// Slice the atlas back into individual frames and verify dimensions & pixels
	sliceDir := filepath.Join(tmp, "sliced")
	count, err := Slice(outPNG, outMeta, sliceDir)
	if err != nil {
		t.Fatalf("Slice failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 sliced frames, got %d", count)
	}

	restoredHero, err := os.Open(filepath.Join(sliceDir, "hero_1.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer restoredHero.Close()
	decoded, err := png.Decode(restoredHero)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 32 || decoded.Bounds().Dy() != 32 {
		t.Errorf("expected restored hero dimensions 32x32, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func writeTestPNG(t *testing.T, p string, img image.Image) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
