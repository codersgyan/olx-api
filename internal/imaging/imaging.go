package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"

	"golang.org/x/image/draw"
)

func Normalize(r io.Reader, maxEdge, quality int) ([]byte, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("imaging: normalise: %w", err)
	}

	dst := scale(src, maxEdge)

	var buf bytes.Buffer

	if err := jpeg.Encode(&buf, dst, &jpeg.Options{
		Quality: quality,
	}); err != nil {
		return nil, fmt.Errorf("imaging: encode: %w", err)
	}

	return buf.Bytes(), nil
}

func scale(src image.Image, maxEdge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	if w > maxEdge || h > maxEdge {
		if w >= h {
			// width is greater = Landscape
			h = int(float64(h) * float64(maxEdge) / float64(w))
			w = maxEdge
		} else {
			// Portrait
			w = int(float64(w) * float64(maxEdge) / float64(h))
			h = maxEdge
		}
	}

	w = max(w, 1)
	h = max(h, 1)

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	return dst
}
