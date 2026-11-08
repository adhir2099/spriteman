package exporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/adhir2099/spriteman/internal/packer"
)

// Format specifies the metadata export format.
type Format string

const (
	FormatJSONHash  Format = "json-hash"
	FormatJSONArray Format = "json-array"
	FormatGodot     Format = "godot"
	FormatCSS       Format = "css"
)

// Size2D represents width and height in metadata.
type Size2D struct {
	W int `json:"w"`
	H int `json:"h"`
}

// Pivot2D represents normalized anchor coordinates.
type Pivot2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// FrameEntry represents a single sprite frame in the exported atlas.
type FrameEntry struct {
	Name             string      `json:"filename,omitempty"`
	Frame            packer.Rect `json:"frame"`
	Rotated          bool        `json:"rotated"`
	Trimmed          bool        `json:"trimmed"`
	SpriteSourceSize packer.Rect `json:"spriteSourceSize"`
	SourceSize       Size2D      `json:"sourceSize"`
	Pivot            Pivot2D     `json:"pivot"`
	AliasOf          string      `json:"aliasOf,omitempty"`
}

// MetaInfo holds metadata about the generated sprite sheet.
type MetaInfo struct {
	App        string  `json:"app"`
	Version    string  `json:"version"`
	Image      string  `json:"image"`
	Format     string  `json:"format"`
	Size       Size2D  `json:"size"`
	Scale      string  `json:"scale"`
	Algorithm  string  `json:"algorithm,omitempty"`
	Efficiency float64 `json:"efficiency,omitempty"`
}

// AtlasDocument represents the full atlas metadata before serialization.
type AtlasDocument struct {
	Frames []FrameEntry
	Meta   MetaInfo
}

// HashAtlasJSON is the PixiJS / Phaser / TexturePacker JSON-Hash schema.
type HashAtlasJSON struct {
	Frames map[string]FrameEntry `json:"frames"`
	Meta   MetaInfo              `json:"meta"`
}

// ArrayAtlasJSON is the TexturePacker JSON-Array schema.
type ArrayAtlasJSON struct {
	Frames []FrameEntry `json:"frames"`
	Meta   MetaInfo     `json:"meta"`
}

// DefaultExtension returns the file extension for a given export Format.
func DefaultExtension(f Format) string {
	switch f {
	case FormatGodot:
		return ".tres"
	case FormatCSS:
		return ".css"
	case FormatJSONArray, FormatJSONHash:
		fallthrough
	default:
		return ".json"
	}
}

// Export serializes doc in the requested Format and writes it to outPath.
func Export(doc *AtlasDocument, format Format, outPath string) error {
	data, err := Render(doc, format)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0644)
}

// Render serializes doc into bytes according to format.
func Render(doc *AtlasDocument, format Format) ([]byte, error) {
	sorted := make([]FrameEntry, len(doc.Frames))
	copy(sorted, doc.Frames)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	switch format {
	case FormatJSONHash:
		framesMap := make(map[string]FrameEntry, len(sorted))
		for _, f := range sorted {
			entry := f
			entry.Name = "" // omit redundant filename inside hash entry
			framesMap[f.Name] = entry
		}
		payload := HashAtlasJSON{
			Frames: framesMap,
			Meta:   doc.Meta,
		}
		return json.MarshalIndent(payload, "", "  ")

	case FormatJSONArray:
		payload := ArrayAtlasJSON{
			Frames: sorted,
			Meta:   doc.Meta,
		}
		return json.MarshalIndent(payload, "", "  ")

	case FormatGodot:
		return renderGodotTres(sorted, doc.Meta), nil

	case FormatCSS:
		return renderCSS(sorted, doc.Meta), nil

	default:
		return nil, fmt.Errorf("unsupported export format %q (expected json-hash, json-array, godot, or css)", format)
	}
}

// ParseJSONFile reads either a JSON-Hash or JSON-Array atlas metadata file from disk.
func ParseJSONFile(path string) (*AtlasDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseJSONBytes(raw)
}

// ParseJSONBytes parses either JSON-Hash or JSON-Array format into an AtlasDocument.
func ParseJSONBytes(raw []byte) (*AtlasDocument, error) {
	var probe struct {
		Frames json.RawMessage `json:"frames"`
		Meta   MetaInfo        `json:"meta"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("parse atlas json: %w", err)
	}

	trimmed := bytes.TrimSpace(probe.Frames)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("missing 'frames' field in atlas JSON")
	}

	var frames []FrameEntry
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &frames); err != nil {
			return nil, fmt.Errorf("parse json-array frames: %w", err)
		}
	} else if trimmed[0] == '{' {
		var frameMap map[string]FrameEntry
		if err := json.Unmarshal(trimmed, &frameMap); err != nil {
			return nil, fmt.Errorf("parse json-hash frames: %w", err)
		}
		frames = make([]FrameEntry, 0, len(frameMap))
		for k, v := range frameMap {
			v.Name = k
			frames = append(frames, v)
		}
		sort.Slice(frames, func(i, j int) bool {
			return frames[i].Name < frames[j].Name
		})
	} else {
		return nil, fmt.Errorf("unexpected 'frames' structure in JSON")
	}

	return &AtlasDocument{
		Frames: frames,
		Meta:   probe.Meta,
	}, nil
}

func renderGodotTres(frames []FrameEntry, meta MetaInfo) []byte {
	var b strings.Builder
	loadSteps := len(frames) + 2
	fmt.Fprintf(&b, "[gd_resource type=\"SpriteFrames\" load_steps=%d format=3]\n\n", loadSteps)
	fmt.Fprintf(&b, "[ext_resource type=\"Texture2D\" path=\"res://%s\" id=\"1_atlas\"]\n\n", meta.Image)

	for i, f := range frames {
		subID := fmt.Sprintf("AtlasTexture_%d", i+1)
		fmt.Fprintf(&b, "[sub_resource type=\"AtlasTexture\" id=%q]\n", subID)
		fmt.Fprintf(&b, "atlas = ExtResource(\"1_atlas\")\n")
		fmt.Fprintf(&b, "region = Rect2(%d, %d, %d, %d)\n", f.Frame.X, f.Frame.Y, f.Frame.Width, f.Frame.Height)
		if f.Trimmed {
			marginW := f.SourceSize.W - f.SpriteSourceSize.Width
			marginH := f.SourceSize.H - f.SpriteSourceSize.Height
			fmt.Fprintf(&b, "margin = Rect2(%d, %d, %d, %d)\n",
				f.SpriteSourceSize.X, f.SpriteSourceSize.Y, marginW, marginH)
		}
		fmt.Fprintf(&b, "filter_clip = true\n\n")
	}

	b.WriteString("[resource]\n")
	b.WriteString("animations = [{\n")
	b.WriteString("\"frames\": [")
	for i := range frames {
		if i > 0 {
			b.WriteString(", ")
		}
		subID := fmt.Sprintf("AtlasTexture_%d", i+1)
		fmt.Fprintf(&b, "{\n\"duration\": 1.0,\n\"texture\": SubResource(%q)\n}", subID)
	}
	b.WriteString("],\n")
	b.WriteString("\"loop\": true,\n")
	b.WriteString("\"name\": &\"default\",\n")
	b.WriteString("\"speed\": 10.0\n")
	b.WriteString("}]\n")

	return []byte(b.String())
}

var nonSlugRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func renderCSS(frames []FrameEntry, meta MetaInfo) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "/* Generated by spriteman (%s) */\n", meta.App)
	fmt.Fprintf(&b, ".sprite {\n")
	fmt.Fprintf(&b, "  display: inline-block;\n")
	fmt.Fprintf(&b, "  background-image: url('%s');\n", meta.Image)
	fmt.Fprintf(&b, "  background-repeat: no-repeat;\n")
	fmt.Fprintf(&b, "  background-size: %dpx %dpx;\n", meta.Size.W, meta.Size.H)
	fmt.Fprintf(&b, "  image-rendering: pixelated;\n")
	fmt.Fprintf(&b, "}\n\n")

	for _, f := range frames {
		slug := strings.Trim(nonSlugRegex.ReplaceAllString(f.Name, "-"), "-")
		if slug == "" {
			slug = "frame"
		}
		fmt.Fprintf(&b, ".sprite-%s {\n", slug)
		fmt.Fprintf(&b, "  width: %dpx;\n", f.Frame.Width)
		fmt.Fprintf(&b, "  height: %dpx;\n", f.Frame.Height)
		fmt.Fprintf(&b, "  background-position: -%dpx -%dpx;\n", f.Frame.X, f.Frame.Y)
		fmt.Fprintf(&b, "}\n")
	}
	return []byte(b.String())
}
