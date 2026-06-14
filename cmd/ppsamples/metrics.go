package main

import (
	"image"
	"math"
)

// magentaResidue는 마젠타에 가까운 불투명 픽셀 수를 센다(배경 제거 잔여물 지표).
func magentaResidue(im *image.NRGBA) int {
	n := 0
	for i := 0; i+3 < len(im.Pix); i += 4 {
		if im.Pix[i+3] <= aThresh {
			continue
		}
		r, g, b := int(im.Pix[i]), int(im.Pix[i+1]), int(im.Pix[i+2])
		if r > 150 && b > 150 && g < 130 {
			n++
		}
	}
	return n
}

// haloPinkish는 마젠타가 섞인 핑크빛 경계(헤일로) 불투명 픽셀 수를 센다.
// 순수 마젠타는 아니지만 R·B가 G보다 확연히 높은(키 색조가 번진) 픽셀.
func haloPinkish(im *image.NRGBA) int {
	n := 0
	for i := 0; i+3 < len(im.Pix); i += 4 {
		if im.Pix[i+3] <= aThresh {
			continue
		}
		r, g, b := int(im.Pix[i]), int(im.Pix[i+1]), int(im.Pix[i+2])
		if r-g > 40 && b-g > 40 && !(r > 200 && b > 200 && g < 80) {
			n++
		}
	}
	return n
}

// distinctColors는 불투명 픽셀의 서로 다른 RGB 개수를 센다(픽셀화 지표).
func distinctColors(im *image.NRGBA) int {
	set := map[uint32]struct{}{}
	for i := 0; i+3 < len(im.Pix); i += 4 {
		if im.Pix[i+3] <= aThresh {
			continue
		}
		k := uint32(im.Pix[i])<<16 | uint32(im.Pix[i+1])<<8 | uint32(im.Pix[i+2])
		set[k] = struct{}{}
	}
	return len(set)
}

// edgeContent는 셀 좌우 가장자리(2px)에 닿은 불투명 픽셀 수를 센다.
// 균등 분할이 포즈를 잘랐을 때 높게 나온다.
func edgeContent(im *image.NRGBA) int {
	w, h := im.Rect.Dx(), im.Rect.Dy()
	n := 0
	for y := 0; y < h; y++ {
		for _, x := range []int{0, 1, w - 2, w - 1} {
			if im.Pix[im.PixOffset(x, y)+3] > aThresh {
				n++
			}
		}
	}
	return n
}

// torsoCenterX는 셔츠색(파랑) 픽셀의 가로 중심을 반환한다(토르소 위치).
func torsoCenterX(im *image.NRGBA) float64 {
	var sx, n float64
	for y := 0; y < im.Rect.Dy(); y++ {
		for x := 0; x < im.Rect.Dx(); x++ {
			i := im.PixOffset(x, y)
			if im.Pix[i+3] <= aThresh {
				continue
			}
			r, g, b := int(im.Pix[i]), int(im.Pix[i+1]), int(im.Pix[i+2])
			if b > 140 && b-r > 40 && g < 180 { // 셔츠 파랑 근사
				sx += float64(x)
				n++
			}
		}
	}
	if n == 0 {
		return 0
	}
	return sx / n
}

// stdDev는 값들의 표준편차를 반환한다(토르소 흔들림 지표).
func stdDev(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	var m float64
	for _, v := range vs {
		m += v
	}
	m /= float64(len(vs))
	var s float64
	for _, v := range vs {
		s += (v - m) * (v - m)
	}
	return math.Sqrt(s / float64(len(vs)))
}

// colAlpha는 컬럼별 불투명 픽셀 수(알파 질량 프로파일)를 반환한다.
func colAlpha(im *image.NRGBA) []float64 {
	w, h := im.Rect.Dx(), im.Rect.Dy()
	p := make([]float64, w)
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			if im.Pix[im.PixOffset(x, y)+3] > aThresh {
				p[x]++
			}
		}
	}
	return p
}

// crossingLines는 균등 N분할선(k·W/n) 중 캐릭터(콘텐츠)를 가로지르는 선의 개수를 센다.
// projection 분할은 빈 gutter를 자르므로 0이지만, equal-split은 포즈 위를 자른다.
func crossingLines(im *image.NRGBA, n int) (cross int, lineMass []float64) {
	p := colAlpha(im)
	w := len(p)
	mx := 0.0
	for _, v := range p {
		if v > mx {
			mx = v
		}
	}
	thresh := 0.06 * mx // 최대 컬럼 질량의 6% 이상이면 "콘텐츠 위"
	for k := 1; k < n; k++ {
		x := k * w / n
		m := p[x]
		lineMass = append(lineMass, m)
		if m > thresh {
			cross++
		}
	}
	return
}

// contentPixels는 불투명 픽셀 수를 센다.
func contentPixels(im *image.NRGBA) int {
	n := 0
	for i := 3; i < len(im.Pix); i += 4 {
		if im.Pix[i] > aThresh {
			n++
		}
	}
	return n
}

// frameBalance는 프레임들의 콘텐츠 픽셀 최소/최대와 (최소 대비) 불균형 비율을 반환한다.
// 균등분할이 빈 칸이나 절반 포즈를 만들면 min이 급감해 비율이 커진다.
func frameBalance(frames []*image.NRGBA) (min, max int, ratio float64) {
	min = 1 << 30
	for _, f := range frames {
		c := contentPixels(f)
		if c < min {
			min = c
		}
		if c > max {
			max = c
		}
	}
	if min <= 0 {
		min = 0
		ratio = 999
	} else {
		ratio = float64(max) / float64(min)
	}
	return
}

func torsoSpread(frames []*image.NRGBA) float64 {
	var xs []float64
	for _, f := range frames {
		xs = append(xs, torsoCenterX(f))
	}
	return stdDev(xs)
}

// contentCentroidX는 셀 내 불투명 픽셀의 가로 질량중심을 반환한다(색 무관).
func contentCentroidX(im *image.NRGBA) float64 {
	var sx, n float64
	for y := 0; y < im.Rect.Dy(); y++ {
		for x := 0; x < im.Rect.Dx(); x++ {
			if im.Pix[im.PixOffset(x, y)+3] > aThresh {
				sx += float64(x)
				n++
			}
		}
	}
	if n == 0 {
		return float64(im.Rect.Dx()) / 2
	}
	return sx / n
}

// centroidSpread는 프레임별 콘텐츠 질량중심 X의 표준편차(앵커 흔들림, 색 무관).
func centroidSpread(frames []*image.NRGBA) float64 {
	var xs []float64
	for _, f := range frames {
		xs = append(xs, contentCentroidX(f))
	}
	return stdDev(xs)
}
