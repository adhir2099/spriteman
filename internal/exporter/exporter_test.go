package exporter

import (
	"strings"
	"testing"

	"github.com/adhir2099/spriteman/internal/packer"
)

func sampleAtlasDoc() *AtlasDocument {
	return &AtlasDocument{
		Frames: []FrameEntry{
			{
				Name:             "hero_idle",
				Frame:            packer.Rect{X: 2, Y: 2, Width: 28, Height: 30},
				Rotated:          false,
				Trimmed:          true,
				SpriteSourceSize: packer.Rect{X: 2, Y: 1, Width: 28, Height: 30},
				SourceSize:       Size2D{W: 32, H: 32},
				Pivot:            Pivot2D{X: 0.5, Y: 0.5},
			},
			{
				Name:             "coin_spin",
				Frame:            packer.Rect{X: 34, Y: 2, Width: 16, Height: 16},
				Rotated:          false,
				Trimmed:          false,
				SpriteSourceSize: packer.Rect{X: 0, Y: 0, Width: 16, Height: 16},
				SourceSize:       Size2D{W: 16, H: 16},
				Pivot:            Pivot2D{X: 0.5, Y: 0.5},
			},
		},
		Meta: MetaInfo{
			App:        "https://github.com/adhir2099/spriteman",
			Version:    "1.0.0",
			Image:      "atlas.png",
			Format:     "RGBA8888",
			Size:       Size2D{W: 64, H: 64},
			Scale:      "1",
			Algorithm:  "maxrects",
			Efficiency: 76.5,
		},
	}
}

func TestRenderAndParseJSONFormats(t *testing.T) {
	doc := sampleAtlasDoc()

	for _, fmtType := range []Format{FormatJSONHash, FormatJSONArray} {
		raw, err := Render(doc, fmtType)
		if err != nil {
			t.Fatalf("Render(%s) failed: %v", fmtType, err)
		}

		parsed, err := ParseJSONBytes(raw)
		if err != nil {
			t.Fatalf("ParseJSONBytes(%s) failed: %v", fmtType, err)
		}
		if len(parsed.Frames) != 2 {
			t.Fatalf("expected 2 frames, got %d", len(parsed.Frames))
		}
		if parsed.Meta.Image != "atlas.png" || parsed.Meta.Size.W != 64 {
			t.Errorf("unexpected meta: %+v", parsed.Meta)
		}
	}
}

func TestRenderGodotAndCSS(t *testing.T) {
	doc := sampleAtlasDoc()

	godotBytes, err := Render(doc, FormatGodot)
	if err != nil {
		t.Fatalf("Render(godot) failed: %v", err)
	}
	godotStr := string(godotBytes)
	if !strings.Contains(godotStr, `[gd_resource type="SpriteFrames"`) || !strings.Contains(godotStr, `region = Rect2(2, 2, 28, 30)`) {
		t.Errorf("unexpected Godot .tres output:\n%s", godotStr)
	}

	cssBytes, err := Render(doc, FormatCSS)
	if err != nil {
		t.Fatalf("Render(css) failed: %v", err)
	}
	cssStr := string(cssBytes)
	if !strings.Contains(cssStr, ".sprite-hero_idle") || !strings.Contains(cssStr, "background-position: -2px -2px;") {
		t.Errorf("unexpected CSS output:\n%s", cssStr)
	}
}
