package accesspolicy

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// A small raster image avoids fonts, scripts and external captcha SDKs. Glyphs
// are pixels, never SVG text or an answer embedded in client-readable markup.
func HomepageCaptchaImage(answer string) (string, error) {
	var noise [512]byte
	if _, err := rand.Read(noise[:]); err != nil {
		return "", err
	}
	img := image.NewRGBA(image.Rect(0, 0, 240, 76))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{242, 246, 253, 255}}, image.Point{}, draw.Src)
	for i := 0; i < 5; i++ {
		for x := 0; x < 240; x++ {
			y := 10 + int(noise[i])%55 + int(6*math.Sin(float64(x)/18+float64(i)))
			img.Set(x, y, color.RGBA{180, 199, 224, 255})
		}
	}
	for index, digit := range answer {
		glyph := image.NewRGBA(image.Rect(0, 0, 8, 15))
		d := font.Drawer{Dst: glyph, Src: image.NewUniform(color.RGBA{28, 48, 84, 255}), Face: basicfont.Face7x13, Dot: fixed.P(0, 12)}
		d.DrawString(string(digit))
		for y := 0; y < 45; y++ {
			for x := 0; x < 24; x++ {
				c := glyph.RGBAAt(x/3, y/3)
				if c.A == 0 {
					continue
				}
				dx := int(3 * math.Sin(float64(y)/11+float64(noise[20+index])))
				img.SetRGBA(12+index*37+x+dx, 12+y+int(noise[30+index]%9)-4, c)
			}
		}
	}
	for i := 50; i < len(noise)-1; i += 2 {
		img.Set(int(noise[i])%240, int(noise[i+1])%76, color.RGBA{124, 151, 192, 255})
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes()), nil
}
