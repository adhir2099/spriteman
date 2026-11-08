# spriteman

[![Go Reference](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![CI](https://github.com/adhir2099/spriteman/actions/workflows/ci.yml/badge.svg)](https://github.com/adhir2099/spriteman/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-amber.svg)](LICENSE)
[![Dependencies](https://img.shields.io/badge/dependencies-zero-brightgreen.svg)](#why-spriteman)

**Fast, zero-dependency 2D Sprite Sheet & Asset Packer CLI written in pure Go**

`spriteman` takes a directory of loose `.png` sprites and packs them into an optimized texture atlas with ready-to-use coordinate metadata for **Godot 4**, **PixiJS**, **Phaser 3**, **TexturePacker**, and **Web CSS**—all in a few milliseconds.

---

## What Does This Do?

When building a 2D game (in **Godot**, **PixiJS**, **Phaser**, or **Unity**) or a website with lots of icons, you usually start with a folder of **loose `.png` files**:

```text
my_game_sprites/
  ├── hero_idle_1.png
  ├── hero_attack.png
  ├── iron_sword.png
  ├── oak_shield.png
  └── slime_green.png
```

If your game tries to load 100 separate image files one by one:
* 🔴 **Slow loading:** The game engine or browser has to make 100 separate file requests
* 🔴 **Laggy performance (Low FPS):** The graphics card (GPU) must switch textures 100 times per frame ("high draw calls"), causing stuttering
* 🔴 **Wasted memory:** Most sprites have large transparent borders around them taking up memory for empty air

### The Solution

`spriteman` packs all your loose sprites together like a master Tetris player:

```text
[ Folder of Loose PNGs ]
  hero.png, sword.png, shield.png...
                  │
                  ▼  run `spriteman pack`
  ┌─────────────────────────────────────────────────────────┐
  │  1. One Single Combined Image (spritesheet.png)         │
  │     All your sprites packed tightly into 1 texture.     │
  │                                                         │
  │  2. A GPS Coordinate Map (spritesheet.json)             │
  │     Tells your game engine:                             │
  │     "sword is at x:0, y:0"                              │
  │     "hero is at x:18, y:0"                              │
  └─────────────────────────────────────────────────────────┘
```

Your game loads **just 1 single image**, runs at silky-smooth 60+ FPS, and consumes **50–70% less memory** thanks to automatic alpha trimming and 8-bit PNG color compression!

---

```text
  spriteman v1.0.0 — 2D Sprite Sheet & Asset Packer
  ──────────────────────────────────────────────────────
  Atlas Output:  examples/dist/rpg_atlas.png (134x136 px)
  Metadata:      examples/dist/rpg_atlas.json (json-hash)
  Algorithm:     maxrects
  Sprites:       10 total (9 packed, 1 deduplicated, 10 alpha-trimmed)
  Efficiency:    60.2% density (excl. padding/extrusion)
  Bundle Size:   2.5 KB -> 658 B (completed in 4.82ms)
  `ASCII visual map`
  ┌────────────────────────────────────────────────┐
  │·DDDDDDDDDDDDDDDDD···AAAAAAAAAAA···GGGGGGGGG····│
  │·DDDDDDDDDDDDDDDDD···AAAAAAAAAAA···GGGGGGGGG····│
  │·DDDDDDDDDDDDDDDDD···AAAAAAAAAAA···GGGGGGGGG····│
  │·DDDDDDDDDDDDDDDDD···AAAAAAAAAAA···HHHHHHHHHHH··│
  │···································HHHHHHHHHHH··│
  │·EEEEEEEEEEEEE··FFFFFFF············HHHHHHHHHHH··│
  │·EEEEEEEEEEEEE··FFFFFFF············BBBBBBBB·····│
  │·EEEEEEEEEEEEE··FFFFFFF············BBBBBBBB·····│
  │·IIIIIIIIIIIIIIIII···CCCCCCC····················│
  │·IIIIIIIIIIIIIIIII···CCCCCCC····················│
  └────────────────────────────────────────────────┘
```

---

## Features

* **Zero External Dependencies:** Built 100% on the Go standard library (`image`, `image/png`, `image/draw`, `crypto/sha256`). Compiles to a single static binary on Windows, macOS, and Linux
* **Automatic Alpha Trimming (`--trim`):** Strips transparent borders around sprites to maximize atlas density while recording exact `spriteSourceSize` and `sourceSize` offsets so engines render sprites in their original positions.
* **SHA-256 Frame Deduplication (`--dedupe`):** Detects identical animation frames, packs the pixel buffer **once**, and aliases duplicate frame names to the same atlas coordinates.
* **WebGL / GPU Edge Extrusion (`--extrude`):** Duplicates border pixels outward to eliminate texture seam bleeding during mipmapping or bilinear filtering.
* **Built-in 8-bit PNG Quantization (`--optimize` / `spriteman crush`):** Converts RGBA atlases into 256-color indexed PNGs with `BestCompression`, shrinking file sizes by **50–75%**.
* **Multi-Engine Exporters (`--format`):**
  * `json-hash` — PixiJS, Phaser 3, TexturePacker Hash JSON
  * `json-array` — TexturePacker Array JSON
  * `godot` — Godot 4 `.tres` `SpriteFrames` / `AtlasTexture` resource
  * `css` — Web CSS sprite classes with `background-position`
* **Round-Trip Atlas Slicer (`spriteman slice`):** Unpacks an existing sprite sheet + JSON back into individual PNG frames (restoring original un-trimmed dimensions).

---

## Installation

### Via `go install`

```bash
go install github.com/adhir2099/spriteman/cmd/spriteman@latest
```

### Build from Source

```bash
git clone https://github.com/adhir2099/spriteman.git
cd spriteman
go build -o spriteman ./cmd/spriteman
```

---

## Quick Start

### 1. Try the Built-in Interactive Demo

Generate 10 sample RPG pixel-art sprites and pack them immediately:

```bash
spriteman demo
```

### 2. Pack a Directory of Sprites

```bash
# Pack for PixiJS / Phaser with 2px padding, 1px edge extrusion, and 8-bit PNG optimization
spriteman pack ./assets/sprites -o ./dist/characters.png --padding 2 --extrude 1 --optimize

# Pack for Godot 4 (.tres resource) with Power-of-Two texture dimensions
spriteman pack ./assets/hero -o ./godot/hero_atlas.png --format godot --pot

# Pack UI icons into a CSS Sprite Sheet for web projects
spriteman pack ./public/icons -o ./public/sprites.png --format css
```

### 3. Slice an Atlas Back into Individual Frames

```bash
spriteman slice ./dist/characters.png --meta ./dist/characters.json -o ./unpacked_frames
```

### 4. Crush / Optimize Existing PNG Assets In-Place

```bash
spriteman crush ./assets/ui --quantize
```

### 5. Inspect an Atlas Metadata File

```bash
spriteman inspect ./dist/characters.json
```

---

## CLI Flags (`spriteman pack`)

| Flag | Default | Description |
| :--- | :---: | :--- |
| `-i`, `--input <dir>` | `.` | Input directory containing `.png` sprites |
| `-o`, `--output <file>` | `spritesheet.png` | Output sprite sheet `.png` path |
| `--meta <file>` | `<output>.<ext>` | Output metadata file path |
| `-f`, `--format <fmt>` | `json-hash` | `json-hash`, `json-array`, `godot`, or `css` |
| `--alg <name>` | `maxrects` | Packing algorithm: `maxrects`, `shelf`, or `grid` |
| `--padding <px>` | `2` | Transparent spacing between packed sprites |
| `--extrude <px>` | `0` | Edge pixel extrusion count to prevent WebGL bleed |
| `--trim` | `true` | Strip transparent margins and preserve `sourceSize` |
| `--dedupe` | `true` | Alias identical frames to the same atlas coordinates |
| `--pot` | `false` | Force Power-of-Two dimensions (`128`, `256`, `512`, `1024`...) |
| `--square` | `false` | Force square dimensions (`width == height`) |
| `--optimize` | `false` | Quantize to 8-bit 256-color indexed PNG |
| `--max-width <px>` | `4096` | Maximum allowed atlas width |
| `--max-height <px>` | `4096` | Maximum allowed atlas height |
| `--config <file>` | — | Load settings from a `spriteman.json` file |
| `--preview` | `true` | Render ASCII layout visualization in terminal |
| `--json` | `false` | Output machine-readable JSON build report for CI |

---

## Configuration File (`spriteman.json`)

Generate a reusable config file for your project:

```bash
spriteman init
```

```json
{
  "input": "./sprites",
  "output": "./dist/spritesheet.png",
  "meta": "./dist/spritesheet.json",
  "format": "json-hash",
  "algorithm": "maxrects",
  "maxWidth": 4096,
  "maxHeight": 4096,
  "padding": 2,
  "extrude": 1,
  "trim": true,
  "dedupe": true,
  "powerOfTwo": false,
  "square": false,
  "optimize": true,
  "recursive": true
}
```

Then build with:

```bash
spriteman pack --config spriteman.json
```

---

## Running Tests

```bash
go test -v ./...
go vet ./...
```

---

## License

Released under the [MIT License](LICENSE) by [Adhir Serrano](https://github.com/adhir2099).
