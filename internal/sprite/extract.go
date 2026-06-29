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
	bottom int          // 베이스라인(콘텐츠 최하단 행, 스트립 좌표)
}

// extractContent는 컬럼 구간 span 안의 불투명 픽셀을 bbox로 잘라냅니다.
// 소유권 추적(연결요소) 없이 구간 내 모든 콘텐츠를 모으므로, 팔다리가 분리되어도
// 한 포즈로 안전하게 합쳐집니다.
func extractContent(strip *image.NRGBA, span colSpan, h int) frameContent {
	minX, minY, maxX, maxY := span.end, h, span.start-1, -1
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
	return frameContent{img: dst, bottom: maxY}
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

	// 캡처 구간: 감지된 포즈 구간에서 시작해 거터로 뻗은 손/소품을 잇되,
	// 작은 빈틈(<maxGap)만 다리 놓고 큰 빈틈 너머(원거리 잔여물)는 버린다.
	// 경계는 이웃과의 거터 중점으로 제한해, 닿은 포즈/이웃 콘텐츠는 넘지 않는다.
	stripW := strip.Rect.Dx()
	hasContent := make([]bool, stripW)
	for x := 0; x < stripW; x++ {
		for y := 0; y < h; y++ {
			if strip.Pix[strip.PixOffset(x, y)+3] > alphaThreshold {
				hasContent[x] = true
				break
			}
		}
	}
	maxGap := stripW / 100
	if maxGap < 10 {
		maxGap = 10
	}
	capture := make([]colSpan, len(segs))
	for i, s := range segs {
		leftBound := 0
		if i > 0 {
			leftBound = (segs[i-1].end + s.start) / 2
		}
		rightBound := stripW
		if i < len(segs)-1 {
			rightBound = (s.end + segs[i+1].start) / 2
		}
		L, R := s.start, s.end
		gap := 0
		for x := s.start - 1; x >= leftBound; x-- {
			if hasContent[x] {
				L = x
				gap = 0
				continue
			}
			gap++
			if gap >= maxGap {
				break
			}
		}
		gap = 0
		for x := s.end; x < rightBound; x++ {
			if hasContent[x] {
				R = x + 1
				gap = 0
				continue
			}
			gap++
			if gap >= maxGap {
				break
			}
		}
		capture[i] = colSpan{L, R}
	}

	var fcs []frameContent
	for _, s := range capture {
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
	maxBodyW, maxBodyH := 1, 1
	for _, g := range fcs {
		bw, bh := bodyExtent(g.img)
		if bw > maxBodyW {
			maxBodyW = bw
		}
		if bh > maxBodyH {
			maxBodyH = bh
		}
	}
	scale := minf(float64(availW)/float64(maxBodyW), float64(availH)/float64(maxBodyH))
	if scale > 1 {
		scale = 1
	}

	for _, g := range fcs {
		// scale은 body extent 기준으로 계산했으므로, 그 외 바운딩 박스 빈 공간은
		// 여유 공간에 맞춰 조정한다.
		boxScale := minf(scale, minf(float64(availW)/float64(g.img.Rect.Dx()), float64(availH)/float64(g.img.Rect.Dy())))
		if boxScale > 1 {
			boxScale = 1
		}
		// 세로 정렬에서는 실제 발/하체 부분(bottom)이 아니라 콘텐츠 하단 기준 사용
		sw := int(float64(g.img.Rect.Dx())*boxScale + 0.5)
		sh := int(float64(g.img.Rect.Dy())*boxScale + 0.5)
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
		// 공통 baseline 보정을 위해 strip 내 콘텐츠 하단 대비 offset을 scale
		contentBaseline := int(float64(baseline-g.bottom)*boxScale + 0.5)

		cell := image.NewNRGBA(image.Rect(0, 0, cellW, cellH))
		// 몸 코어(80% 질량 창)의 중심을 셀 중앙에 정렬한다. 전체 무게중심이 아니라
		// 코어를 쓰는 이유: 보주처럼 한쪽으로 뻗은 소품이 무게중심을 끌어 몸통 루트가
		// 프레임마다 흔들리는 것을 막기 위함(회전 시 제자리 회전 보장).
		coreX := bodyCoreCenterX(g.img, 0.80)
		left := int(float64(cellW)/2 - coreX*boxScale + 0.5)
		if left < 0 {
			left = 0
		}
		if left+sw > cellW {
			left = cellW - sw
		}
		top := cellH - margin - contentBaseline - sh
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

// bodyExtent는 알파 질량 80%를 포함하는 최소 크기를 "실제 바디" extent로 반환합니다.
// 길게 뻗은 팔다리 outlier가 스케일을 과대 산정하는 것을 막습니다.
func bodyExtent(img *image.NRGBA) (int, int) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 {
		return 1, 1
	}
	alphaX := make([]float64, w)
	alphaY := make([]float64, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := float64(img.Pix[img.PixOffset(x, y)+3])
			alphaX[x] += a
			alphaY[y] += a
		}
	}
	cutX := cumulativeExtent(alphaX, 0.80)
	cutY := cumulativeExtent(alphaY, 0.80)
	if cutX < 1 {
		cutX = 1
	}
	if cutY < 1 {
		cutY = 1
	}
	return cutX, cutY
}

// cumulativeExtent는 질량 누적 비율 massFrac를 커버하는 가장 좁은 연속 구간의 길이를 반환합니다.
func cumulativeExtent(mass []float64, massFrac float64) int {
	total := 0.0
	for _, v := range mass {
		total += v
	}
	if total == 0 {
		return 0
	}
	target := total * massFrac
	n := len(mass)
	best := n
	left := 0
	cur := 0.0
	for right := 0; right < n; right++ {
		cur += mass[right]
		for cur >= target {
			if span := right - left + 1; span < best {
				best = span
			}
			cur -= mass[left]
			left++
		}
	}
	return best
}

// bodyCoreCenterX는 80% 알파 질량을 담는 최소폭 수평 창의 중심 X(로컬 좌표)를 반환합니다.
// 얇게 뻗은 팔/소품(<20% 질량)을 배제한 "몸통 코어"라, 정렬 기준으로 쓰면 회전 중
// 루트(중심) 위치가 프레임마다 일정하게 유지됩니다.
func bodyCoreCenterX(img *image.NRGBA, massFrac float64) float64 {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	alphaX := make([]float64, w)
	total := 0.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := float64(img.Pix[img.PixOffset(x, y)+3])
			alphaX[x] += a
			total += a
		}
	}
	if total <= 0 {
		return float64(w) / 2
	}
	target := total * massFrac
	bestL, bestR, best := 0, w-1, w+1
	left := 0
	cur := 0.0
	for right := 0; right < w; right++ {
		cur += alphaX[right]
		for cur >= target && left <= right {
			if span := right - left + 1; span < best {
				best, bestL, bestR = span, left, right
			}
			cur -= alphaX[left]
			left++
		}
	}
	return (float64(bestL) + float64(bestR) + 1) / 2
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
