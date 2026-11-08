package packer

import (
	"fmt"
	"math"
	"sort"
)

type shelfRow struct {
	y      int
	height int
	usedX  int
}

func tryPackShelf(items []Item, binW, binH, margin int) (map[string]Rect, int, int, bool) {
	sorted := make([]Item, len(items))
	copy(sorted, items)
	// Shelf FFDH sorts primarily by height descending
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Height != sorted[j].Height {
			return sorted[i].Height > sorted[j].Height
		}
		if sorted[i].Width != sorted[j].Width {
			return sorted[i].Width > sorted[j].Width
		}
		return sorted[i].ID < sorted[j].ID
	})

	var shelves []shelfRow
	placements := make(map[string]Rect, len(sorted))
	usedW, usedH := 0, 0
	currentY := 0

	for _, it := range sorted {
		reqW := it.Width + margin*2
		reqH := it.Height + margin*2
		if reqW > binW || reqH > binH {
			return nil, 0, 0, false
		}

		placed := false
		for idx := range shelves {
			if shelves[idx].usedX+reqW <= binW && reqH <= shelves[idx].height {
				x := shelves[idx].usedX + margin
				y := shelves[idx].y + margin
				placements[it.ID] = Rect{
					X:      x,
					Y:      y,
					Width:  it.Width,
					Height: it.Height,
				}
				shelves[idx].usedX += reqW
				if shelves[idx].usedX > usedW {
					usedW = shelves[idx].usedX
				}
				placed = true
				break
			}
		}

		if !placed {
			if currentY+reqH > binH {
				return nil, 0, 0, false
			}
			shelves = append(shelves, shelfRow{
				y:      currentY,
				height: reqH,
				usedX:  reqW,
			})
			placements[it.ID] = Rect{
				X:      margin,
				Y:      currentY + margin,
				Width:  it.Width,
				Height: it.Height,
			}
			if reqW > usedW {
				usedW = reqW
			}
			currentY += reqH
			if currentY > usedH {
				usedH = currentY
			}
		}
	}

	return placements, usedW, usedH, true
}

func packGrid(items []Item, opts Options, margin int) (map[string]Rect, int, int, error) {
	// Sort alphabetically by ID for predictable sprite sheet frame indices
	sorted := make([]Item, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	cellW, cellH := 1, 1
	for _, it := range sorted {
		if it.Width+margin*2 > cellW {
			cellW = it.Width + margin*2
		}
		if it.Height+margin*2 > cellH {
			cellH = it.Height + margin*2
		}
	}

	cols := int(math.Ceil(math.Sqrt(float64(len(sorted)))))
	if cols < 1 {
		cols = 1
	}
	if cols*cellW > opts.MaxWidth {
		cols = opts.MaxWidth / cellW
		if cols < 1 {
			return nil, 0, 0, fmt.Errorf("%w: grid cell width %d exceeds max width %d",
				ErrAtlasOverflow, cellW, opts.MaxWidth)
		}
	}

	rows := (len(sorted) + cols - 1) / cols
	usedW := cols * cellW
	usedH := rows * cellH

	if usedW > opts.MaxWidth || usedH > opts.MaxHeight {
		return nil, 0, 0, fmt.Errorf("%w: grid (%dx%d) exceeds max (%dx%d)",
			ErrAtlasOverflow, usedW, usedH, opts.MaxWidth, opts.MaxHeight)
	}

	placements := make(map[string]Rect, len(sorted))
	for idx, it := range sorted {
		col := idx % cols
		row := idx / cols
		cellX := col * cellW
		cellY := row * cellH
		// Center sprite within uniform grid cell
		offsetX := (cellW - it.Width) / 2
		offsetY := (cellH - it.Height) / 2
		placements[it.ID] = Rect{
			X:      cellX + offsetX,
			Y:      cellY + offsetY,
			Width:  it.Width,
			Height: it.Height,
		}
	}

	return placements, usedW, usedH, nil
}
