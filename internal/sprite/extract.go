package sprite

import (
	"fmt"
	"image"

	xdraw "golang.org/x/image/draw"
)

const alphaThreshold = 10 // 이 값 이하의 알파는 빈(투명) 픽셀로 취급

// frameContent는 스트립 좌표계에서 추출한 한 포즈의 콘텐츠입니다.
type frameContent struct {
	img    *image.NRGBA // bbox로 자른 콘텐츠
	minX   int
	cx     float64 // 알파 가중 질량 중심 X (스트립 좌표)
	bottom int     // 베이스라인(콘텐츠 최하단 행, 스트립 좌표)
}

// extractContent는 컬럼 구간 span 안의 불투명 픽셀을 bbox로 잘라냅니다.
// 소유권 추적(연결요소) 없이 구간 내 모든 콘텐츠를 모으므로, 팔다리가 분리되어도
// 한 포즈로 안전하게 합쳐집니다.
func extractContent(strip *image.NRGBA, span colSpan, h int) frameContent {
	minX, minY, maxX, maxY := span.end, h, span.start-1, -1
	var sumWX, sumW float64
	for x := span.start; x < span.end; x++ {
		for y := 0; y < h; y++ {
			a := strip.Pix[strip.PixOffset(x, y)+3]
			if a <= alphaThreshold {
				continue
			}
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
			sumWX += float64(x) * float64(a)
			sumW += float64(a)
		}
	}
	if maxX < minX || maxY < minY {
		return frameContent{}
	}
	gw, gh := maxX-minX+1, maxY-minY+1
	dst := image.NewNRGBA(image.Rect(0, 0, gw, gh))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			si := strip.PixOffset(x, y)
			if strip.Pix[si+3] <= alphaThreshold {
				continue
			}
			di := dst.PixOffset(x-minX, y-minY)
			copy(dst.Pix[di:di+4], strip.Pix[si:si+4])
		}
	}
	cx := float64(minX+maxX+1) / 2
	if sumW > 0 {
		cx = sumWX / sumW
	}
	return frameContent{img: dst, minX: minX, cx: cx, bottom: maxY}
}

// ExtractFrames는 투명 배경 스트립에서 포즈를 투영 분할로 검출해 셀 크기 프레임으로
// 만듭니다. 모든 프레임에 공통 스케일을 적용하고, 질량 중심으로 수평 정렬하며,
// 공통 베이스라인 기준으로 수직 오프셋(점프 호 등)을 보존합니다.
func ExtractFrames(strip *image.NRGBA, expected, cellW, cellH, margin int) ExtractResult {
	res := ExtractResult{Expected: expected}
	segs, natural := segmentStrip(strip, expected)
	if len(segs) == 0 {
		res.Warnings = append(res.Warnings, "이미지에서 캐릭터를 찾지 못했습니다. 다시 생성해 주세요.")
		return res
	}
	h := strip.Rect.Dy()

	var fcs []frameContent
	for _, s := range segs {
		fc := extractContent(strip, s, h)
		if fc.img != nil {
			fcs = append(fcs, fc)
		}
	}
	if len(fcs) == 0 {
		res.Warnings = append(res.Warnings, "유효한 포즈를 찾지 못했습니다. 다시 생성해 주세요.")
		return res
	}

	// 공통 베이스라인 + 공유 스케일
	baseline := 0
	for _, g := range fcs {
		if g.bottom > baseline {
			baseline = g.bottom
		}
	}
	availW := cellW - margin*2
	availH := cellH - margin*2
	if availW < 8 || availH < 8 {
		availW, availH = cellW, cellH
	}
	maxW, maxEffH := 1, 1
	for _, g := range fcs {
		offset := baseline - g.bottom
		if g.img.Rect.Dx() > maxW {
			maxW = g.img.Rect.Dx()
		}
		if eff := g.img.Rect.Dy() + offset; eff > maxEffH {
			maxEffH = eff
		}
	}
	scale := minf(float64(availW)/float64(maxW), float64(availH)/float64(maxEffH))
	if scale > 1 {
		scale = 1
	}

	for _, g := range fcs {
		sw := int(float64(g.img.Rect.Dx())*scale + 0.5)
		sh := int(float64(g.img.Rect.Dy())*scale + 0.5)
		if sw < 1 {
			sw = 1
		}
		if sh < 1 {
			sh = 1
		}
		scaled := g.img
		if sw != g.img.Rect.Dx() || sh != g.img.Rect.Dy() {
			scaled = image.NewNRGBA(image.Rect(0, 0, sw, sh))
			xdraw.CatmullRom.Scale(scaled, scaled.Rect, g.img, g.img.Rect, xdraw.Over, nil)
		}
		offset := int(float64(baseline-g.bottom)*scale + 0.5)

		cell := image.NewNRGBA(image.Rect(0, 0, cellW, cellH))
		// 질량 중심이 셀 중앙에 오도록 수평 배치 (팔다리가 한쪽으로 뻗어도
		// 면적이 큰 몸통이 지배해 프레임 간 흔들림이 적음).
		left := int(float64(cellW)/2 - (g.cx-float64(g.minX))*scale + 0.5)
		if left < 0 {
			left = 0
		}
		if left+sw > cellW {
			left = cellW - sw
		}
		top := cellH - margin - offset - sh
		if top < 0 {
			top = 0
		}
		xdraw.Copy(cell, image.Point{X: left, Y: top}, scaled, scaled.Rect, xdraw.Over, nil)
		res.Frames = append(res.Frames, cell)
	}

	res.Found = natural
	if natural != expected {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("기대한 %d개와 다른 %d개의 포즈가 감지되었습니다. 포즈가 겹쳤거나 누락됐을 수 있어 재생성을 권장합니다.", expected, natural))
	}
	return res
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
