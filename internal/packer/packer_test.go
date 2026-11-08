package packer

import (
	"errors"
	"fmt"
	"testing"
)

func TestNextPowerOfTwo(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{31, 32},
		{32, 32},
		{33, 64},
		{500, 512},
	}
	for _, tc := range cases {
		if got := NextPowerOfTwo(tc.in); got != tc.want {
			t.Errorf("NextPowerOfTwo(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestPackAlgorithmsNoOverlap(t *testing.T) {
	items := []Item{
		{ID: "hero_idle", Width: 48, Height: 64},
		{ID: "hero_run_1", Width: 52, Height: 64},
		{ID: "hero_run_2", Width: 50, Height: 62},
		{ID: "slime_1", Width: 32, Height: 24},
		{ID: "slime_2", Width: 34, Height: 22},
		{ID: "coin", Width: 16, Height: 16},
		{ID: "sword", Width: 12, Height: 44},
		{ID: "shield", Width: 28, Height: 30},
		{ID: "boss_dragon", Width: 128, Height: 96},
	}

	algorithms := []Algorithm{AlgorithmMaxRects, AlgorithmShelf, AlgorithmGrid}
	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			res, err := Pack(items, Options{
				Algorithm:  alg,
				MaxWidth:   1024,
				MaxHeight:  1024,
				Padding:    2,
				Extrude:    1,
				PowerOfTwo: true,
			})
			if err != nil {
				t.Fatalf("Pack(%s) unexpected error: %v", alg, err)
			}

			if len(res.Placements) != len(items) {
				t.Fatalf("got %d placements, want %d", len(res.Placements), len(items))
			}

			// Verify power-of-two constraint
			if NextPowerOfTwo(res.Width) != res.Width || NextPowerOfTwo(res.Height) != res.Height {
				t.Errorf("dimensions %dx%d are not powers of two", res.Width, res.Height)
			}

			// Verify bounds and zero overlap (including margin)
			margin := 2 + 1*2
			placedList := make([]Placement, 0, len(res.Placements))
			for id, r := range res.Placements {
				if r.X < margin || r.Y < margin || r.X+r.Width+margin > res.Width || r.Y+r.Height+margin > res.Height {
					t.Errorf("item %s rect %+v violates atlas bounds %dx%d with margin %d", id, r, res.Width, res.Height, margin)
				}
				placedList = append(placedList, Placement{ID: id, Rect: r})
			}

			for i := 0; i < len(placedList); i++ {
				for j := i + 1; j < len(placedList); j++ {
					a := placedList[i].Rect
					b := placedList[j].Rect
					// Expand each by margin to ensure padding + extrude is respected
					paddedA := Rect{X: a.X - margin, Y: a.Y - margin, Width: a.Width + margin*2, Height: a.Height + margin*2}
					if paddedA.Intersects(b) {
						t.Errorf("overlap detected between %s (%+v) and %s (%+v)",
							placedList[i].ID, a, placedList[j].ID, b)
					}
				}
			}
		})
	}
}

func TestPackOverflow(t *testing.T) {
	var items []Item
	for i := 0; i < 20; i++ {
		items = append(items, Item{
			ID:     fmt.Sprintf("tile_%d", i),
			Width:  64,
			Height: 64,
		})
	}

	_, err := Pack(items, Options{
		Algorithm: AlgorithmMaxRects,
		MaxWidth:  128,
		MaxHeight: 128,
	})
	if !errors.Is(err, ErrAtlasOverflow) {
		t.Fatalf("expected ErrAtlasOverflow, got %v", err)
	}
}
