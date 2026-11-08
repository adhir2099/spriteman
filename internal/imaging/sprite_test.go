package imaging

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/adhir2099/spriteman/internal/packer"
)

func TestLoadSpriteTrimmingAndHash(t *testing.T) {
	tmpDir := t.TempDir()
	pathA := filepath.Join(tmpDir, "sprite_a.png")
	pathB := filepath.Join(tmpDir, "sprite_b.png")

	// Create 32x32 image with a 10x12 opaque box at (5, 7)
	imgA := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 7; y < 7+12; y++ {
		for x := 5; x < 5+10; x++ {
			imgA.SetNRGBA(x, y, color.NRGBA{R: 255, G: 80, B: 40, A: 255})
		}
	}
	saveTempPNG(t, pathA, imgA)

	// Create 48x48 image with the exact same 10x12 opaque box at (12, 15)
	imgB := image.NewNRGBA(image.Rect(0, 0, 48, 48))
	for y := 15; y < 15+12; y++ {
		for x := 12; x < 12+10; x++ {
			imgB.SetNRGBA(x, y, color.NRGBA{R: 255, G: 80, B: 40, A: 255})
		}
	}
	saveTempPNG(t, pathB, imgB)

	spA, err := LoadSprite(pathA, "a", true)
	if err != nil {
		t.Fatalf("LoadSprite A failed: %v", err)
	}
	if !spA.WasTrimmed || spA.TrimX != 5 || spA.TrimY != 7 || spA.TrimmedW != 10 || spA.TrimmedH != 12 {
		t.Errorf("unexpected trim for A: %+v", spA)
	}

	spB, err := LoadSprite(pathB, "b", true)
	if err != nil {
		t.Fatalf("LoadSprite B failed: %v", err)
	}
	if spA.Hash != spB.Hash {
		t.Errorf("expected identical trimmed pixel hashes, got %s vs %s", spA.Hash, spB.Hash)
	}
}

func TestDrawSpriteWithExtrude(t *testing.T) {
	dst := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	red := color.NRGBA{R: 220, G: 20, B: 60, A: 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetNRGBA(x, y, red)
		}
	}

	DrawSpriteWithExtrude(dst, src, packer.Rect{X: 4, Y: 4, Width: 4, Height: 4}, 2)

	// Extruded region should cover [2..9, 2..9]
	for y := 2; y <= 9; y++ {
		for x := 2; x <= 9; x++ {
			if got := dst.NRGBAAt(x, y); got != red {
				t.Fatalf("expected extruded pixel at (%d,%d) to be %v, got %v", x, y, red, got)
			}
		}
	}
	if got := dst.NRGBAAt(1, 1); got.A != 0 {
		t.Errorf("expected pixel outside extrude at (1,1) to be transparent, got %v", got)
	}
}

func saveTempPNG(t *testing.T, p string, img image.Image) {
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
