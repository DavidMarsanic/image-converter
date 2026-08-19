package engine

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/HugoSmits86/nativewebp"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

// detectFormat sniffs data's actual format from its magic bytes, never
// from a filename — a renamed or misnamed upload should still convert
// correctly. Deliberately not using image.RegisterFormat/image.Decode's
// built-in dispatch: nativewebp registers under the bare "RIFF" prefix
// (broader than "RIFFxxxxWEBP"), which risks shadowing other RIFF-family
// files depending on unspecified package init order. Sniffing explicitly
// here sidesteps that entirely and gives each format its own clear error.
func detectFormat(data []byte) (Format, error) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return JPEG, nil
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return PNG, nil
	case bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")):
		return GIF, nil
	case bytes.HasPrefix(data, []byte("BM")):
		return BMP, nil
	case bytes.HasPrefix(data, []byte("II*\x00")) || bytes.HasPrefix(data, []byte("MM\x00*")):
		return TIFF, nil
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return WebP, nil
	default:
		return "", fmt.Errorf("unrecognized image format — supported: JPEG, PNG, WebP, GIF, BMP, TIFF")
	}
}

func decodeConfig(format Format, data []byte) (image.Config, error) {
	r := bytes.NewReader(data)
	switch format {
	case JPEG:
		return jpeg.DecodeConfig(r)
	case PNG:
		return png.DecodeConfig(r)
	case GIF:
		return gif.DecodeConfig(r)
	case BMP:
		return bmp.DecodeConfig(r)
	case TIFF:
		return tiff.DecodeConfig(r)
	case WebP:
		return webp.DecodeConfig(r)
	default:
		return image.Config{}, fmt.Errorf("unsupported format %q", format)
	}
}

func decode(format Format, data []byte) (image.Image, error) {
	r := bytes.NewReader(data)
	switch format {
	case JPEG:
		return jpeg.Decode(r)
	case PNG:
		return png.Decode(r)
	case GIF:
		return gif.Decode(r)
	case BMP:
		return bmp.Decode(r)
	case TIFF:
		return tiff.Decode(r)
	case WebP:
		return webp.Decode(r)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func encode(w io.Writer, img image.Image, format Format, quality int) error {
	switch format {
	case JPEG:
		if quality <= 0 || quality > 100 {
			quality = 85
		}
		return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	case PNG:
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		return enc.Encode(w, img)
	case GIF:
		return gif.Encode(w, img, &gif.Options{NumColors: 256})
	case BMP:
		return bmp.Encode(w, img)
	case TIFF:
		return tiff.Encode(w, img, &tiff.Options{Compression: tiff.Deflate})
	case WebP:
		// nativewebp only writes lossless VP8L — there's no pure-Go lossy
		// WebP encoder available without cgo, which this whole family of
		// apps avoids to keep the CGO_ENABLED=0 cross-compile working. So
		// unlike JPEG, quality has no effect on WebP output here.
		return nativewebp.Encode(w, img, &nativewebp.Options{CompressionLevel: nativewebp.DefaultCompression})
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
