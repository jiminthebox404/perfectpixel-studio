package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"

	xdraw "golang.org/x/image/draw"
)

const aThresh = 10 // sprite 패키지와 동일한 빈 픽셀 기준

// jpegRoundTrip은 합성 이미지를 JPEG로 인코딩/디코딩해 실제 AI 출력처럼
// 4:2:0 색차 subsampling 아티팩트(블록 노이즈, 경계 색번짐)를 입힌다.
func jpegRoundTrip(im *image.NRGBA, q int) *image.NRGBA {
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, im, &jpeg.Options{Quality: q})
	dec, _ := jpeg.Decode(&buf)
	out := image.NewNRGBA(im.Rect)
	xdraw.Copy(out, image.Point{}, dec, dec.Bounds(), xdraw.Src, nil)
	return out
}

// naiveMatte는 "기술 없음" 배경 제거: 순수 RGB 거리 임계값 하나로만 자른다.
// soft alpha, despill, flood fill, morphology 없음 → 헤일로/잔여물이 남는다.
func naiveMatte(src image.Image, key color.NRGBA, tol float64) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	xdraw.Copy(out, image.Point{}, src, b, xdraw.Src, nil)
	for i := 0; i+3 < len(out.Pix); i += 4 {
		dr := float64(out.Pix[i]) - float64(key.R)
		dg := float64(out.Pix[i+1]) - float64(key.G)
		db := float64(out.Pix[i+2]) - float64(key.B)
		if dr*dr+dg*dg+db*db <= tol*tol {
			out.Pix[i+3] = 0 // 배경 → 투명 (RGB는 그대로 둬 헤일로 잔존)
		}
	}
	return out
}

// equalSplitExtract는 "기술 없음" 프레임 추출: 스트립을 n등분해 각 칸의
// 콘텐츠를 bbox 중심으로 셀에 배치한다. (projection/DP 없음, centroid 없음)
func equalSplitExtract(strip *image.NRGBA, n, cellW, cellH, margin int) []*image.NRGBA {
	w, h := strip.Rect.Dx(), strip.Rect.Dy()
	var frames []*image.NRGBA
	for k := 0; k < n; k++ {
		x0 := w * k / n
		x1 := w * (k + 1) / n
		frames = append(frames, placeBBoxCenter(strip, x0, x1, h, cellW, cellH, margin))
	}
	return frames
}

// placeBBoxCenter는 [x0,x1) 구간 콘텐츠를 bbox 중심 기준으로 셀 중앙에 둔다.
func placeBBoxCenter(strip *image.NRGBA, x0, x1, h, cellW, cellH, margin int) *image.NRGBA {
	minX, minY, maxX, maxY := x1, h, x0-1, -1
	for x := x0; x < x1; x++ {
		for y := 0; y < h; y++ {
			if strip.Pix[strip.PixOffset(x, y)+3] > aThresh {
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
	cell := newCanvas(cellW, cellH, color.NRGBA{0, 0, 0, 0})
	if maxX < minX {
		return cell
	}
	gw, gh := maxX-minX+1, maxY-minY+1
	avail := cellH - margin*2
	scale := float64(avail) / float64(gh)
	if scale > 1 {
		scale = 1
	}
	sw, sh := int(float64(gw)*scale+0.5), int(float64(gh)*scale+0.5)
	src := image.NewNRGBA(image.Rect(0, 0, gw, gh))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			si := strip.PixOffset(x, y)
			if strip.Pix[si+3] > aThresh {
				di := src.PixOffset(x-minX, y-minY)
				copy(src.Pix[di:di+4], strip.Pix[si:si+4])
			}
		}
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, sw, sh))
	xdraw.CatmullRom.Scale(scaled, scaled.Rect, src, src.Rect, xdraw.Over, nil)
	left := (cellW - sw) / 2 // bbox 중심을 셀 중앙에 (← 기술 없음의 핵심)
	top := cellH - margin - sh
	xdraw.Copy(cell, image.Point{X: left, Y: top}, scaled, scaled.Rect, xdraw.Over, nil)
	return cell
}

// overOn은 src를 bg 위에 알파 합성한 새 이미지를 만든다(투명 결과 시연용).
func overOn(bg, src *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(bg.Rect)
	copy(out.Pix, bg.Pix)
	xdraw.Copy(out, image.Point{}, src, src.Rect, xdraw.Over, nil)
	return out
}

// paste는 src를 dst의 (x,y)에 붙인다.
func paste(dst, src *image.NRGBA, x, y int) {
	xdraw.Copy(dst, image.Point{X: x, Y: y}, src, src.Rect, xdraw.Over, nil)
}

// resizeW는 가로 폭 targetW로 비율 유지 리샘플한다.
func resizeW(im *image.NRGBA, targetW int) *image.NRGBA {
	if im.Rect.Dx() == targetW {
		return im
	}
	h := im.Rect.Dy() * targetW / im.Rect.Dx()
	if h < 1 {
		h = 1
	}
	out := image.NewNRGBA(image.Rect(0, 0, targetW, h))
	xdraw.CatmullRom.Scale(out, out.Rect, im, im.Rect, xdraw.Over, nil)
	return out
}
