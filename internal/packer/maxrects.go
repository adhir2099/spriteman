package packer

import "math"

// maxRectsBin maintains a list of maximal free rectangles inside a fixed-size bin.
type maxRectsBin struct {
	width     int
	height    int
	freeRects []Rect
}

func newMaxRectsBin(width, height int) *maxRectsBin {
	return &maxRectsBin{
		width:  width,
		height: height,
		freeRects: []Rect{
			{X: 0, Y: 0, Width: width, Height: height},
		},
	}
}

// insert finds the best position for a w x h rectangle using Best Short Side Fit (BSSF).
func (b *maxRectsBin) insert(w, h int) (Rect, bool) {
	bestNode := Rect{}
	bestShortSideFit := math.MaxInt
	bestLongSideFit := math.MaxInt
	found := false

	for _, free := range b.freeRects {
		if free.Width >= w && free.Height >= h {
			leftoverHoriz := free.Width - w
			leftoverVert := free.Height - h
			shortSideFit := min(leftoverHoriz, leftoverVert)
			longSideFit := max(leftoverHoriz, leftoverVert)

			if shortSideFit < bestShortSideFit ||
				(shortSideFit == bestShortSideFit && longSideFit < bestLongSideFit) ||
				(shortSideFit == bestShortSideFit && longSideFit == bestLongSideFit &&
					(free.Y < bestNode.Y || (free.Y == bestNode.Y && free.X < bestNode.X))) {
				bestNode = Rect{X: free.X, Y: free.Y, Width: w, Height: h}
				bestShortSideFit = shortSideFit
				bestLongSideFit = longSideFit
				found = true
			}
		}
	}

	if !found {
		return Rect{}, false
	}

	b.placeRect(bestNode)
	return bestNode, true
}

func (b *maxRectsBin) placeRect(placed Rect) {
	var nextFree []Rect
	for _, free := range b.freeRects {
		if !free.Intersects(placed) {
			nextFree = append(nextFree, free)
			continue
		}

		// Split free rectangle around the placed rectangle
		if placed.X < free.X+free.Width && placed.X+placed.Width > free.X {
			// New node above placed rect
			if placed.Y > free.Y && placed.Y < free.Y+free.Height {
				nextFree = append(nextFree, Rect{
					X:      free.X,
					Y:      free.Y,
					Width:  free.Width,
					Height: placed.Y - free.Y,
				})
			}
			// New node below placed rect
			if placed.Y+placed.Height < free.Y+free.Height {
				nextFree = append(nextFree, Rect{
					X:      free.X,
					Y:      placed.Y + placed.Height,
					Width:  free.Width,
					Height: (free.Y + free.Height) - (placed.Y + placed.Height),
				})
			}
		}

		if placed.Y < free.Y+free.Height && placed.Y+placed.Height > free.Y {
			// New node to the left of placed rect
			if placed.X > free.X && placed.X < free.X+free.Width {
				nextFree = append(nextFree, Rect{
					X:      free.X,
					Y:      free.Y,
					Width:  placed.X - free.X,
					Height: free.Height,
				})
			}
			// New node to the right of placed rect
			if placed.X+placed.Width < free.X+free.Width {
				nextFree = append(nextFree, Rect{
					X:      placed.X + placed.Width,
					Y:      free.Y,
					Width:  (free.X + free.Width) - (placed.X + placed.Width),
					Height: free.Height,
				})
			}
		}
	}

	b.freeRects = pruneFreeList(nextFree)
}

// pruneFreeList removes redundant rectangles that are completely contained within another free rectangle.
func pruneFreeList(rects []Rect) []Rect {
	pruned := make([]Rect, 0, len(rects))
	for i := 0; i < len(rects); i++ {
		contained := false
		for j := 0; j < len(rects); j++ {
			if i == j {
				continue
			}
			if rects[j].Contains(rects[i]) {
				// If two rectangles are identical, keep only the one with lower index
				if rects[i] == rects[j] && i < j {
					continue
				}
				contained = true
				break
			}
		}
		if !contained {
			pruned = append(pruned, rects[i])
		}
	}
	return pruned
}

func tryPackMaxRects(items []Item, binW, binH, margin int) (map[string]Rect, int, int, bool) {
	bin := newMaxRectsBin(binW, binH)
	placements := make(map[string]Rect, len(items))
	usedW, usedH := 0, 0

	for _, it := range items {
		paddedW := it.Width + margin*2
		paddedH := it.Height + margin*2

		node, ok := bin.insert(paddedW, paddedH)
		if !ok {
			return nil, 0, 0, false
		}

		actual := Rect{
			X:      node.X + margin,
			Y:      node.Y + margin,
			Width:  it.Width,
			Height: it.Height,
		}
		placements[it.ID] = actual

		if node.X+node.Width > usedW {
			usedW = node.X + node.Width
		}
		if node.Y+node.Height > usedH {
			usedH = node.Y + node.Height
		}
	}

	return placements, usedW, usedH, true
}
