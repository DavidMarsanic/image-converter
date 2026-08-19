# Image Converter

Batch-convert images between JPEG, PNG, WebP, GIF, BMP, and TIFF —
entirely on your machine, nothing uploaded anywhere. Opens as its own
window.

## Features

- Drop in any number of images and convert them all to one target format
  in a single pass.
- Optional quality setting for JPEG output, and an optional max-dimension
  cap (resizes down, never up) for shrinking large photos.
- Converted files are saved as `<name>.<ext>` next to any existing file
  of that name (never overwritten — `name (2).ext`, and so on), to your
  Downloads folder by default or a folder you choose.
- Transparent images (PNG, WebP) converting to a format without alpha
  (JPEG, GIF, BMP, TIFF) get flattened onto white first, instead of the
  black-where-transparent-used-to-be result a naive conversion produces.

## Requirements

**A Chromium-based browser already installed**: Google Chrome, Chromium,
Brave, Microsoft Edge, or Arc — renders the app's own UI window.

## Notes

- WebP output is always lossless — there's no pure-Go lossy WebP encoder
  available without cgo, and this app (like the rest of this family)
  avoids cgo to keep its cross-platform build simple. Lossless WebP files
  are larger than a comparable lossy JPEG but still typically smaller
  than PNG.
- GIF output is a single still frame — this isn't an animation tool (see
  GIF Maker for that).

## License

MIT — see [LICENSE](LICENSE).
