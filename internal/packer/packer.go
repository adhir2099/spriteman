package packer

import (
	"errors"
	"fmt"
	"sort"
)

// Algorithm identifies the bin-packing strategy.
type Algorithm string

const (
	AlgorithmMaxRects Algorithm = "maxrects"
	AlgorithmShelf    Algorithm = "shelf"
	AlgorithmGrid     Algorithm = "grid"
)

// ErrAtlasOverflow is returned when the input sprites cannot fit within MaxWidth x MaxHeight.
var ErrAtlasOverflow = errors.New("sprites exceed maximum atlas dimensions")

// Item represents a rectangular sprite to be packed.
type Item struct {
	ID     string
	Width  int
	Height int
}

// Rect represents an axis-aligned rectangle in atlas coordinates.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"w"`
	Height int `json:"h"`
}

// Intersects reports whether r and other overlap.
func (r Rect) Intersects(other Rect) bool {
	return r.X < other.X+other.Width &&
		r.X+r.Width > other.X &&
		r.Y < other.Y+other.Height &&
		r.Y+r.Height > other.Y
}

// Contains reports whether other is completely inside r.
func (r Rect) Contains(other Rect) bool {
	return other.X >= r.X &&
		other.Y >= r.Y &&
		other.X+other.Width <= r.X+r.Width &&
		other.Y+other.Height <= r.Y+r.Height
}

// Placement maps an Item ID to its final packed position inside the atlas.
type Placement struct {
	ID   string
	Rect Rect
}

// Options configures the packing constraints and layout algorithm.
type Options struct {
	Algorithm   Algorithm
	MaxWidth    int
	MaxHeight   int
	Padding     int  // Transparent spacing between sprites
	Extrude     int  // Edge pixel extrusion per side
	PowerOfTwo  bool // Force output dimensions to powers of 2
	Square      bool // Force output width == height
}

// Result holds the final atlas dimensions, sprite placements, and efficiency metrics.
type Result struct {
	Width      int
	Height     int
	Placements map[string]Rect
	UsedArea   int
	TotalArea  int
	Efficiency float64
}

// NextPowerOfTwo returns the smallest power of two >= n.
func NextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// Pack arranges items into a 2D atlas according to opts.
func Pack(items []Item, opts Options) (*Result, error) {
	if len(items) == 0 {
		return &Result{
			Width:      1,
			Height:     1,
			Placements: make(map[string]Rect),
		}, nil
	}

	if opts.MaxWidth <= 0 {
		opts.MaxWidth = 4096
	}
	if opts.MaxHeight <= 0 {
		opts.MaxHeight = 4096
	}
	if opts.Algorithm == "" {
		opts.Algorithm = AlgorithmMaxRects
	}

	margin := opts.Padding + (opts.Extrude * 2)

	// Validate single-item bounds and compute total area
	totalSpriteArea := 0
	paddedArea := 0
	maxItemW := 0
	maxItemH := 0

	sorted := make([]Item, len(items))
	copy(sorted, items)

	// Sort by largest longest-side, then area, then ID for deterministic output
	sort.Slice(sorted, func(i, j int) bool {
		maxI := max(sorted[i].Width, sorted[i].Height)
		maxJ := max(sorted[j].Width, sorted[j].Height)
		if maxI != maxJ {
			return maxI > maxJ
		}
		areaI := sorted[i].Width * sorted[i].Height
		areaJ := sorted[j].Width * sorted[j].Height
		if areaI != areaJ {
			return areaI > areaJ
		}
		return sorted[i].ID < sorted[j].ID
	})

	for _, it := range sorted {
		if it.Width <= 0 || it.Height <= 0 {
			continue
		}
		reqW := it.Width + margin*2
		reqH := it.Height + margin*2
		if reqW > opts.MaxWidth || reqH > opts.MaxHeight {
			return nil, fmt.Errorf("%w: sprite %q (%dx%d with margin %d) exceeds max (%dx%d)",
				ErrAtlasOverflow, it.ID, reqW, reqH, margin, opts.MaxWidth, opts.MaxHeight)
		}
		totalSpriteArea += it.Width * it.Height
		paddedArea += (it.Width + margin) * (it.Height + margin)
		if reqW > maxItemW {
			maxItemW = reqW
		}
		if reqH > maxItemH {
			maxItemH = reqH
		}
	}

	var placements map[string]Rect
	var boundsW, boundsH int
	var err error

	switch opts.Algorithm {
	case AlgorithmGrid:
		placements, boundsW, boundsH, err = packGrid(sorted, opts, margin)
	case AlgorithmShelf:
		placements, boundsW, boundsH, err = packWithGrowingBins(sorted, opts, margin, maxItemW, maxItemH, paddedArea, tryPackShelf)
	case AlgorithmMaxRects:
		fallthrough
	default:
		placements, boundsW, boundsH, err = packWithGrowingBins(sorted, opts, margin, maxItemW, maxItemH, paddedArea, tryPackMaxRects)
	}

	if err != nil {
		return nil, err
	}

	finalW, finalH := finalizeDimensions(boundsW, boundsH, opts)
	if finalW > opts.MaxWidth || finalH > opts.MaxHeight {
		return nil, fmt.Errorf("%w: final dimensions %dx%d exceed max %dx%d",
			ErrAtlasOverflow, finalW, finalH, opts.MaxWidth, opts.MaxHeight)
	}

	totalArea := finalW * finalH
	eff := 0.0
	if totalArea > 0 {
		eff = (float64(totalSpriteArea) / float64(totalArea)) * 100.0
	}

	return &Result{
		Width:      finalW,
		Height:     finalH,
		Placements: placements,
		UsedArea:   totalSpriteArea,
		TotalArea:  totalArea,
		Efficiency: eff,
	}, nil
}

type binTryFunc func(items []Item, binW, binH, margin int) (map[string]Rect, int, int, bool)

func packWithGrowingBins(items []Item, opts Options, margin, minW, minH, estArea int, tryFn binTryFunc) (map[string]Rect, int, int, error) {
	side := 16
	for side*side < estArea {
		side += 4
	}

	// Test multiple aspect-ratio widths to find the tightest bounding box area
	multipliers := []float64{0.85, 0.95, 1.0, 1.1, 1.2, 1.35, 1.5, 1.75, 2.0}
	var bestPlaced map[string]Rect
	bestW, bestH := 0, 0
	bestFinalArea := 0

	for _, mult := range multipliers {
		candW := max(minW, min(int(float64(side)*mult), opts.MaxWidth))
		candH := opts.MaxHeight
		if opts.Square {
			candH = candW
		}
		if candH < minH {
			continue
		}

		if placed, usedW, usedH, ok := tryFn(items, candW, candH, margin); ok {
			fw, fh := finalizeDimensions(usedW, usedH, opts)
			if fw > opts.MaxWidth || fh > opts.MaxHeight {
				continue
			}
			area := fw * fh
			// Prefer smaller area, and break ties by closeness to square aspect ratio
			if bestPlaced == nil || area < bestFinalArea || (area == bestFinalArea && absDiff(fw, fh) < absDiff(bestW, bestH)) {
				bestPlaced = placed
				bestW = usedW
				bestH = usedH
				bestFinalArea = area
			}
		}
	}

	if bestPlaced != nil {
		return bestPlaced, bestW, bestH, nil
	}

	// Fallback: try full MaxWidth x MaxHeight
	if placed, usedW, usedH, ok := tryFn(items, opts.MaxWidth, opts.MaxHeight, margin); ok {
		return placed, usedW, usedH, nil
	}

	return nil, 0, 0, fmt.Errorf("%w: could not fit %d sprites within %dx%d",
		ErrAtlasOverflow, len(items), opts.MaxWidth, opts.MaxHeight)
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func finalizeDimensions(w, h int, opts Options) (int, int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if opts.PowerOfTwo {
		w = NextPowerOfTwo(w)
		h = NextPowerOfTwo(h)
	}
	if opts.Square {
		side := max(w, h)
		w, h = side, side
	}
	return w, h
}
