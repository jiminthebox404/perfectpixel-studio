package main

import (
	"fmt"
	"image"
	"image/color"

	"perfectpixel/internal/sprite"
)

// scaleDown은 정수 배율 f로 nearest 축소한다.
func scaleDown(im *image.NRGBA, f int) *image.NRGBA {
	if f < 2 {
		return im
	}
	w, h := im.Rect.Dx()/f, im.Rect.Dy()/f
	out := newCanvas(w, h, color.NRGBA{255, 255, 255, 255})
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := im.PixOffset(x*f, y*f)
			setPx(out, x, y, color.NRGBA{im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]})
		}
	}
	return out
}

// scaleUp은 정수 배율 f로 nearest 확대한다(픽셀 격자 확대용).
func scaleUp(im *image.NRGBA, f int) *image.NRGBA {
	w, h := im.Rect.Dx()*f, im.Rect.Dy()*f
	out := newCanvas(w, h, color.NRGBA{0, 0, 0, 0})
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := im.PixOffset(x/f, y/f)
			setPx(out, x, y, color.NRGBA{im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]})
		}
	}
	return out
}

// scaleRow는 프레임들을 gray 위에 올려 f배 축소 후 가로로 잇는다.
func scaleRow(frames []*image.NRGBA, over *image.NRGBA, f int) *image.NRGBA {
	var panels []*image.NRGBA
	for _, fr := range frames {
		panels = append(panels, scaleDown(overOn(over, fr), f))
	}
	return hstack(2, color.NRGBA{200, 204, 210, 255}, panels...)
}

func blend(dst, src, a uint8) uint8 {
	return uint8((int(dst)*(255-int(a)) + int(src)*int(a)) / 255)
}

// onionSkin은 모든 프레임을 반투명하게 겹쳐 토르소 정렬 상태를 보여준다.
func onionSkin(frames []*image.NRGBA, over *image.NRGBA) *image.NRGBA {
	if len(frames) == 0 {
		return over
	}
	cw, ch := frames[0].Rect.Dx(), frames[0].Rect.Dy()
	acc := newCanvas(cw, ch, color.NRGBA{0, 0, 0, 0})
	a := uint8(100)
	for _, f := range frames {
		for i := 0; i+3 < len(f.Pix); i += 4 {
			if f.Pix[i+3] <= aThresh {
				continue
			}
			acc.Pix[i] = blend(acc.Pix[i], f.Pix[i], a)
			acc.Pix[i+1] = blend(acc.Pix[i+1], f.Pix[i+1], a)
			acc.Pix[i+2] = blend(acc.Pix[i+2], f.Pix[i+2], a)
			if na := uint16(acc.Pix[i+3]) + uint16(a); na > 255 {
				acc.Pix[i+3] = 255
			} else {
				acc.Pix[i+3] = uint8(na)
			}
		}
	}
	out := overOn(over, acc)
	cx := cw / 2
	for y := 0; y < ch; y++ {
		setPx(out, cx, y, color.NRGBA{220, 40, 40, 255})
	}
	return out
}

func cloneNRGBA(im *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(im.Rect)
	copy(out.Pix, im.Pix)
	return out
}

func cropRect(im *image.NRGBA, x, y, w, h int) *image.NRGBA {
	out := newCanvas(w, h, color.NRGBA{255, 255, 255, 255})
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			i := im.PixOffset(x+xx, y+yy)
			if i >= 0 && i+3 < len(im.Pix) {
				setPx(out, xx, yy, color.NRGBA{im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]})
			}
		}
	}
	return out
}

func u8c(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// shadeGradient는 비마젠타 픽셀에 대각 그라데이션 음영을 입혀 색 수를 늘린다.
func shadeGradient(im *image.NRGBA) {
	w, h := im.Rect.Dx(), im.Rect.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := im.PixOffset(x, y)
			r, g, b := im.Pix[i], im.Pix[i+1], im.Pix[i+2]
			if r > 200 && b > 200 && g < 80 {
				continue
			}
			f := 0.6 + 0.4*float64(x+y)/float64(w+h)
			im.Pix[i] = u8c(float64(r) * f)
			im.Pix[i+1] = u8c(float64(g) * f)
			im.Pix[i+2] = u8c(float64(b) * f)
		}
	}
}

// ---------- 3. centroid 정렬 ----------

func demoCentroid(matte *image.NRGBA, n int) {
	fmt.Println("[3] alpha-weighted centroid 정렬 — 실제 AI 스트립")
	W := cell * n
	without := equalSplitExtract(matte, n, cell, cell, 16)
	with := sprite.ExtractFrames(matte, n, cell, cell, 16).Frames

	swo := centroidSpread(without)
	swi := centroidSpread(with)
	fmt.Printf("  콘텐츠 질량중심 가로 표준편차(작을수록 안정): 없음=%.1f  있음=%.1f\n", swo, swi)

	gray := newCanvas(cell, cell, colBG)
	woRow := scaleRow(without, gray, 2)
	wiRow := scaleRow(with, gray, 2)
	woOnion := scaleDown(onionSkin(without, gray), 2)
	wiOnion := scaleDown(onionSkin(with, gray), 2)

	onions := hstack(20, color.NRGBA{255, 255, 255, 255},
		labeledPanel(woOnion, fmt.Sprintf("WITHOUT: bbox-center  (anchor std %.1fpx)", swo), colWithout),
		labeledPanel(wiOnion, fmt.Sprintf("WITH: centroid  (anchor std %.1fpx)", swi), colWith),
	)
	title := titleBar(W/2, "3. Frame alignment: alpha-weighted centroid (extract.go)")
	out := vstack(6, color.NRGBA{255, 255, 255, 255},
		title,
		captionBar(W/2, "Onion-skin of all frames (red = cell center). Tight overlap = stable anchor.", colInk),
		onions,
		labeledPanel(woRow, "WITHOUT: bbox-center placement (torso jitters)", colWithout),
		labeledPanel(wiRow, "WITH: centroid placement (torso locked)", colWith),
	)
	save("03-centroid.png", out)
}

// ---------- 4. pixelize + quantize ----------

func demoPixelize(raw *image.NRGBA) {
	fmt.Println("[4] palette quantize + grid snap — 실제 AI 캐릭터")
	frame := sprite.ExtractFrames(sprite.RemoveBackground(raw), 1, cell, cell, 16).Frames
	if len(frame) == 0 {
		fmt.Println("  추출 실패")
		return
	}
	without := cloneNRGBA(frame[0])
	withFrames := []*image.NRGBA{cloneNRGBA(frame[0])}
	sprite.PixelPostProcess(withFrames, 32)
	with := withFrames[0]

	cwo := distinctColors(without)
	cwi := distinctColors(with)
	fmt.Printf("  서로 다른 색 수: 없음=%d  있음=%d\n", cwo, cwi)

	gray := newCanvas(cell, cell, colBG)
	cropWO := scaleUp(cropRect(overOn(gray, without), cell/2-40, 20, 80, 80), 4)
	cropWI := scaleUp(cropRect(overOn(gray, with), cell/2-40, 20, 80, 80), 4)

	full := hstack(20, color.NRGBA{255, 255, 255, 255},
		labeledPanel(overOn(gray, without), fmt.Sprintf("WITHOUT: raw  (%d colors)", cwo), colWithout),
		labeledPanel(overOn(gray, with), fmt.Sprintf("WITH: quantize+snap  (%d colors)", cwi), colWith),
	)
	zoom := hstack(20, color.NRGBA{255, 255, 255, 255},
		labeledPanel(cropWO, "WITHOUT zoom 4x (soft AA, gradient)", colWithout),
		labeledPanel(cropWI, "WITH zoom 4x (crisp pixel grid)", colWith),
	)
	title := titleBar(cell*2+40, "4. Pixel-art: shared palette + grid snap (quantize.go/pixelize.go)")
	save("04-pixelize.png", vstack(6, color.NRGBA{255, 255, 255, 255}, title, full, zoom))
}
