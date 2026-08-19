// Package engine decodes and re-encodes images between formats — entirely
// in memory, no external tool, no network access. Unlike Photo Privacy
// Cleaner's byte-surgical metadata strip, format conversion is inherently a
// decode/re-encode: there's no way to turn a PNG into a JPEG without
// touching every pixel.
package engine

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
)

// Format is one of the image formats this app can read or write.
type Format string

const (
	JPEG Format = "jpeg"
	PNG  Format = "png"
	WebP Format = "webp"
	GIF  Format = "gif"
	BMP  Format = "bmp"
	TIFF Format = "tiff"
)

// formatsWithAlpha are the formats this app writes with their alpha channel
// intact. Everything else gets flattened onto a white background first —
// asking Go's stdlib JPEG/BMP/TIFF/GIF encoders to write a transparent
// pixel silently produces black instead, since they read premultiplied
// RGBA and a fully transparent pixel premultiplies to (0,0,0,0).
var formatsWithAlpha = map[Format]bool{PNG: true, WebP: true}

// Extension is the file extension (with leading dot) written for a format.
func (f Format) Extension() string {
	switch f {
	case JPEG:
		return ".jpg"
	default:
		return "." + string(f)
	}
}

// Info is what Inspect reports about an image without decoding its pixels.
type Info struct {
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Bytes  int    `json:"bytes"`
}

// Inspect reports the format and dimensions of data without decoding the
// full image — just enough to show a preview label before conversion.
func Inspect(data []byte) (*Info, error) {
	format, err := detectFormat(data)
	if err != nil {
		return nil, err
	}
	cfg, err := decodeConfig(format, data)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", format, err)
	}
	return &Info{Format: string(format), Width: cfg.Width, Height: cfg.Height, Bytes: len(data)}, nil
}

// Options controls how Convert re-encodes an image.
type Options struct {
	Format Format
	// Quality is 1-100, JPEG output only (WebP here is always lossless —
	// see the note in encodeWebP). Ignored for every other format.
	Quality int
	// MaxDimension caps the longer side, preserving aspect ratio. 0 keeps
	// the original size.
	MaxDimension int
}

// Result is one converted image.
type Result struct {
	Data          []byte
	Extension     string
	Width, Height int
}

// Convert decodes data, optionally resizes it, and re-encodes it as
// opts.Format.
func Convert(data []byte, opts Options) (*Result, error) {
	srcFormat, err := detectFormat(data)
	if err != nil {
		return nil, err
	}
	img, err := decode(srcFormat, data)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", srcFormat, err)
	}

	if opts.MaxDimension > 0 {
		img = resizeToMax(img, opts.MaxDimension)
	}
	if !formatsWithAlpha[opts.Format] && hasAlpha(img) {
		img = flattenOntoWhite(img)
	}

	var buf bytes.Buffer
	if err := encode(&buf, img, opts.Format, opts.Quality); err != nil {
		return nil, fmt.Errorf("writing %s: %w", opts.Format, err)
	}

	b := img.Bounds()
	return &Result{Data: buf.Bytes(), Extension: opts.Format.Extension(), Width: b.Dx(), Height: b.Dy()}, nil
}

// hasAlpha reports whether img's color model can represent transparency —
// not whether any pixel actually uses it, since scanning every pixel just
// to decide whether flattening is needed would cost as much as the
// flatten itself.
func hasAlpha(img image.Image) bool {
	switch img.ColorModel() {
	case color.RGBAModel, color.NRGBAModel, color.RGBA64Model, color.NRGBA64Model:
		return true
	default:
		return false
	}
}

func flattenOntoWhite(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, b, img, b.Min, draw.Over)
	return dst
}

// resizeToMax scales img down so its longer side is maxDim, preserving
// aspect ratio. Never upscales — a "max dimension" is a ceiling, not a
// target. Uses CatmullRom (a sharp, high-quality resampler well suited to
// photographic downscaling) rather than nearest/linear.
func resizeToMax(img image.Image, maxDim int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return img
	}

	var nw, nh int
	if w >= h {
		nw = maxDim
		nh = int(math.Round(float64(h) * float64(maxDim) / float64(w)))
	} else {
		nh = maxDim
		nw = int(math.Round(float64(w) * float64(maxDim) / float64(h)))
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	return dst
}
