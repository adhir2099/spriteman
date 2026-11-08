package imaging

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"sort"

	"github.com/adhir2099/spriteman/internal/packer"
)

// SpriteData holds a loaded sprite's pixel data, original dimensions, trim offsets, and content hash.
type SpriteData struct {
	ID          string
	FilePath    string
	Image       *image.NRGBA // Trimmed (or original) pixel buffer starting at (0,0)
	SourceW     int          // Original un-trimmed width
	SourceH     int          // Original un-trimmed height
	TrimX       int          // X offset of trimmed region inside original image
	TrimY       int          // Y offset of trimmed region inside original image
	TrimmedW    int          // Width after trimming
	TrimmedH    int          // Height after trimming
	WasTrimmed  bool
	Hash        string       // SHA-256 of dimensions + pixel bytes for deduplication
	FileSize    int64        // Original file size in bytes
}

// LoadSprite reads a PNG file from disk, optionally trims transparent borders, and hashes its pixels.
func LoadSprite(filePath, id string, trimAlpha bool) (*SpriteData, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", filePath, err)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	rawImg, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode png %s: %w", filePath, err)
	}

	nrgba := ToNRGBA(rawImg)
	srcW := nrgba.Bounds().Dx()
	srcH := nrgba.Bounds().Dy()

	trimX, trimY := 0, 0
	trimW, trimH := srcW, srcH
	wasTrimmed := false
	finalImg := nrgba

	if trimAlpha {
		bounds, nonEmpty := findOpaqueBounds(nrgba)
		if !nonEmpty {
			// Fully transparent sprite: keep 1x1 transparent pixel at (0,0)
			trimX, trimY = 0, 0
			trimW, trimH = 1, 1
			wasTrimmed = srcW > 1 || srcH > 1
			finalImg = image.NewNRGBA(image.Rect(0, 0, 1, 1))
		} else if bounds.Min.X > 0 || bounds.Min.Y > 0 || bounds.Max.X < srcW || bounds.Max.Y < srcH {
			trimX = bounds.Min.X
			trimY = bounds.Min.Y
			trimW = bounds.Dx()
			trimH = bounds.Dy()
			wasTrimmed = true
			finalImg = cropNormalized(nrgba, bounds)
		}
	}

	hash := computePixelHash(finalImg)

	return &SpriteData{
		ID:         id,
		FilePath:   filePath,
		Image:      finalImg,
		SourceW:    srcW,
		SourceH:    srcH,
		TrimX:      trimX,
		TrimY:      trimY,
		TrimmedW:   trimW,
		TrimmedH:   trimH,
		WasTrimmed: wasTrimmed,
		Hash:       hash,
		FileSize:   info.Size(),
	}, nil
}

// ToNRGBA converts any image.Image into a normalized *image.NRGBA with origin (0,0).
func ToNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// findOpaqueBounds returns the tightest bounding rectangle where alpha > 0.
func findOpaqueBounds(img *image.NRGBA) (image.Rectangle, bool) {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	minX, minY := w, h
	maxX, maxY := -1, -1

	for y := 0; y < h; y++ {
		rowOffset := y * img.Stride
		for x := 0; x < w; x++ {
			alpha := img.Pix[rowOffset+x*4+3]
			if alpha > 0 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if maxX < minX || maxY < minY {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

func cropNormalized(src *image.NRGBA, r image.Rectangle) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), src, r.Min, draw.Src)
	return dst
}

func computePixelHash(img *image.NRGBA) string {
	h := sha256.New()
	var dims [8]byte
	binary.LittleEndian.PutUint32(dims[0:4], uint32(img.Bounds().Dx()))
	binary.LittleEndian.PutUint32(dims[4:8], uint32(img.Bounds().Dy()))
	h.Write(dims[:])
	h.Write(img.Pix)
	return hex.EncodeToString(h.Sum(nil))
}

// DrawSpriteWithExtrude blits src onto dst at dstRect and extrudes border pixels outward by extrudepx.
func DrawSpriteWithExtrude(dst *image.NRGBA, src *image.NRGBA, dstRect packer.Rect, extrude int) {
	target := image.Rect(dstRect.X, dstRect.Y, dstRect.X+dstRect.Width, dstRect.Y+dstRect.Height)
	draw.Draw(dst, target, src, image.Point{0, 0}, draw.Src)

	if extrude <= 0 {
		return
	}

	w := dstRect.Width
	h := dstRect.Height

	// Top & bottom edge extrusion
	for e := 1; e <= extrude; e++ {
		topY := dstRect.Y - e
		botY := dstRect.Y + h - 1 + e
		for x := 0; x < w; x++ {
			if topY >= 0 {
				dst.SetNRGBA(dstRect.X+x, topY, src.NRGBAAt(x, 0))
			}
			if botY < dst.Bounds().Dy() {
				dst.SetNRGBA(dstRect.X+x, botY, src.NRGBAAt(x, h-1))
			}
		}
	}

	// Left & right edge extrusion
	for e := 1; e <= extrude; e++ {
		leftX := dstRect.X - e
		rightX := dstRect.X + w - 1 + e
		for y := 0; y < h; y++ {
			if leftX >= 0 {
				dst.SetNRGBA(leftX, dstRect.Y+y, src.NRGBAAt(0, y))
			}
			if rightX < dst.Bounds().Dx() {
				dst.SetNRGBA(rightX, dstRect.Y+y, src.NRGBAAt(w-1, y))
			}
		}
	}

	// 4 Corners extrusion
	tl := src.NRGBAAt(0, 0)
	tr := src.NRGBAAt(w-1, 0)
	bl := src.NRGBAAt(0, h-1)
	br := src.NRGBAAt(w-1, h-1)
	for dy := 1; dy <= extrude; dy++ {
		for dx := 1; dx <= extrude; dx++ {
			dst.SetNRGBA(dstRect.X-dx, dstRect.Y-dy, tl)
			dst.SetNRGBA(dstRect.X+w-1+dx, dstRect.Y-dy, tr)
			dst.SetNRGBA(dstRect.X-dx, dstRect.Y+h-1+dy, bl)
			dst.SetNRGBA(dstRect.X+w-1+dx, dstRect.Y+h-1+dy, br)
		}
	}
}

// SavePNG encodes img to outPath using BestCompression and optional 256-color palette quantization.
func SavePNG(outPath string, img *image.NRGBA, quantize bool) (int64, error) {
	var toEncode image.Image = img
	if quantize {
		toEncode = QuantizeToPaletted(img)
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, toEncode); err != nil {
		return 0, fmt.Errorf("encode png: %w", err)
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		return 0, fmt.Errorf("write png %s: %w", outPath, err)
	}

	return int64(buf.Len()), nil
}

// QuantizeToPaletted converts an NRGBA image to an 8-bit indexed Paletted image (up to 256 colors).
// If the image already has <= 256 unique colors (common in pixel art), it preserves 100% exact colors!
// Otherwise, it quantizes to the top 255 most frequent reduced-bit colors + transparent index 0.
func QuantizeToPaletted(src *image.NRGBA) *image.Paletted {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	// Fast check: collect exact unique colors up to 256
	exactMap := make(map[color.NRGBA]uint8, 256)
	palette := make(color.Palette, 0, 256)
	palette = append(palette, color.NRGBA{0, 0, 0, 0})
	exactMap[color.NRGBA{0, 0, 0, 0}] = 0

	exceededExact := false
	for y := 0; y < h && !exceededExact; y++ {
		row := y * src.Stride
		for x := 0; x < w; x++ {
			i := row + x*4
			a := src.Pix[i+3]
			if a == 0 {
				continue
			}
			c := color.NRGBA{R: src.Pix[i], G: src.Pix[i+1], B: src.Pix[i+2], A: a}
			if _, exists := exactMap[c]; !exists {
				if len(palette) >= 256 {
					exceededExact = true
					break
				}
				exactMap[c] = uint8(len(palette))
				palette = append(palette, c)
			}
		}
	}

	if !exceededExact {
		dst := image.NewPaletted(b, palette)
		for y := 0; y < h; y++ {
			row := y * src.Stride
			for x := 0; x < w; x++ {
				i := row + x*4
				a := src.Pix[i+3]
				if a == 0 {
					dst.SetColorIndex(x, y, 0)
					continue
				}
				c := color.NRGBA{R: src.Pix[i], G: src.Pix[i+1], B: src.Pix[i+2], A: a}
				dst.SetColorIndex(x, y, exactMap[c])
			}
		}
		return dst
	}

	// Histogram-based popularity quantization on 5-bit channels
	type bucket struct {
		key            uint32
		count          int
		sumR, sumG, sumB, sumA int
	}
	buckets := make(map[uint32]*bucket, 1024)
	for y := 0; y < h; y++ {
		row := y * src.Stride
		for x := 0; x < w; x++ {
			i := row + x*4
			a := src.Pix[i+3]
			if a < 8 {
				continue
			}
			r, g, bl := src.Pix[i], src.Pix[i+1], src.Pix[i+2]
			key := (uint32(r>>3) << 15) | (uint32(g>>3) << 10) | (uint32(bl>>3) << 5) | uint32(a>>3)
			bk, ok := buckets[key]
			if !ok {
				bk = &bucket{key: key}
				buckets[key] = bk
			}
			bk.count++
			bk.sumR += int(r)
			bk.sumG += int(g)
			bk.sumB += int(bl)
			bk.sumA += int(a)
		}
	}

	sortedBuckets := make([]*bucket, 0, len(buckets))
	for _, bk := range buckets {
		sortedBuckets = append(sortedBuckets, bk)
	}
	sort.Slice(sortedBuckets, func(i, j int) bool {
		if sortedBuckets[i].count != sortedBuckets[j].count {
			return sortedBuckets[i].count > sortedBuckets[j].count
		}
		return sortedBuckets[i].key < sortedBuckets[j].key
	})

	quantPalette := make(color.Palette, 1, 256)
	quantPalette[0] = color.NRGBA{0, 0, 0, 0}
	limit := min(255, len(sortedBuckets))
	for i := 0; i < limit; i++ {
		bk := sortedBuckets[i]
		quantPalette = append(quantPalette, color.NRGBA{
			R: uint8(bk.sumR / bk.count),
			G: uint8(bk.sumG / bk.count),
			B: uint8(bk.sumB / bk.count),
			A: uint8(bk.sumA / bk.count),
		})
	}

	dst := image.NewPaletted(b, quantPalette)
	cache := make(map[uint32]uint8, 1024)
	for y := 0; y < h; y++ {
		row := y * src.Stride
		for x := 0; x < w; x++ {
			i := row + x*4
			a := src.Pix[i+3]
			if a < 8 {
				dst.SetColorIndex(x, y, 0)
				continue
			}
			r, g, bl := src.Pix[i], src.Pix[i+1], src.Pix[i+2]
			key := (uint32(r>>3) << 15) | (uint32(g>>3) << 10) | (uint32(bl>>3) << 5) | uint32(a>>3)
			if idx, ok := cache[key]; ok {
				dst.SetColorIndex(x, y, idx)
				continue
			}
			idx := uint8(quantPalette.Index(color.NRGBA{R: r, G: g, B: bl, A: a}))
			cache[key] = idx
			dst.SetColorIndex(x, y, idx)
		}
	}
	return dst
}
