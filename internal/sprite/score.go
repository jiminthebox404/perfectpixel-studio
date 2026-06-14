package sprite

import "image"

// 품질 점수 + 정체성 지표. 평가 하니스가 생성 결과를 기계적으로 채점하는 데 씁니다.

// QualityMetrics는 한 상태(애니메이션) 결과의 측정값 묶음입니다.
type QualityMetrics struct {
	Expected     int     `json:"expected"`
	Found        int     `json:"found"`
	Errors       int     `json:"errors"`
	Warnings     int     `json:"warnings"`
	Motion       float64 `json:"motion"`       // 인접 프레임 평균 변화율 (0=정지)
	IdentityHash float64 `json:"identityHash"` // 프레임 간 dHash 유사도 0~1 (1=동일 구조)
	Score        int     `json:"score"`        // 종합 0~100
	Label        string  `json:"label"`
}

// ScoreFrames는 추출/검사 결과를 종합해 0~100 품질 점수를 산출합니다.
// 가중치: 프레임 수 정확도(가장 큼) → 심각 오류 → 정체성/모션 → 경고.
func ScoreFrames(frames []*image.NRGBA, expected int, insp InspectResult, motion float64) QualityMetrics {
	m := QualityMetrics{
		Expected: expected,
		Found:    len(frames),
		Errors:   len(insp.Errors),
		Warnings: len(insp.Warnings),
		Motion:   motion,
	}
	m.IdentityHash = IdentityConsistency(frames)

	score := 100.0
	// 프레임 수: 기대치와의 차이에 비례한 큰 감점
	if m.Found != expected {
		diff := m.Found - expected
		if diff < 0 {
			diff = -diff
		}
		score -= 35 + 10*float64(diff)
	}
	// 심각 오류 / 경고
	score -= 13 * float64(m.Errors)
	score -= 3 * float64(m.Warnings)
	// 정지(애니메이션이 움직이지 않음): 2프레임 이상인데 변화 거의 없음
	if m.Found >= 2 && motion < 0.01 {
		score -= 12
	}
	// 정체성 붕괴: 프레임 간 구조가 과하게 다르면 추가 감점
	if m.Found >= 2 && m.IdentityHash < 0.55 {
		score -= 10
	}
	if score < 0 {
		score = 0
	}
	m.Score = int(score + 0.5)
	switch {
	case m.Score >= 85:
		m.Label = "excellent"
	case m.Score >= 70:
		m.Label = "good"
	case m.Score >= 50:
		m.Label = "fair"
	default:
		m.Label = "poor"
	}
	return m
}

// dHash는 9×8 그레이스케일 차분 해시(64bit)를 계산합니다.
// 인접 픽셀 밝기 비교라 색/조명 변화에 둔감하고 구조(포즈/실루엣)에 민감합니다.
func dHash(img *image.NRGBA) uint64 {
	const w, h = 9, 8
	small := image.NewNRGBA(image.Rect(0, 0, w, h))
	// 최근접 다운샘플 (정확도보다 속도 — 해시 안정성에는 충분)
	sw, sh := img.Rect.Dx(), img.Rect.Dy()
	if sw == 0 || sh == 0 {
		return 0
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := x * sw / w
			sy := y * sh / h
			i := img.PixOffset(sx, sy)
			r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
			// 투명 픽셀은 검정으로 — 실루엣이 해시에 반영됨
			lum := uint8(0)
			if a > alphaThreshold {
				lum = uint8((299*int(r) + 587*int(g) + 114*int(b)) / 1000)
			}
			di := small.PixOffset(x, y)
			small.Pix[di] = lum
		}
	}
	var hash uint64
	bit := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w-1; x++ {
			l := small.Pix[small.PixOffset(x, y)]
			r := small.Pix[small.PixOffset(x+1, y)]
			if l > r {
				hash |= 1 << uint(bit)
			}
			bit++
		}
	}
	return hash
}

func hamming(a, b uint64) int {
	x := a ^ b
	c := 0
	for x != 0 {
		x &= x - 1
		c++
	}
	return c
}

// IdentityConsistency는 인접 프레임 dHash 해밍거리 평균을 0~1 유사도로 변환합니다.
// 1=구조 동일, 0=완전 상이. 포즈가 바뀌므로 1은 아니며, 급락 시 캐릭터 변형 신호.
func IdentityConsistency(frames []*image.NRGBA) float64 {
	if len(frames) < 2 {
		return 1
	}
	hashes := make([]uint64, len(frames))
	for i, f := range frames {
		hashes[i] = dHash(f)
	}
	var total, pairs float64
	for i := 1; i < len(frames); i++ {
		total += float64(hamming(hashes[i-1], hashes[i]))
		pairs++
	}
	avg := total / pairs // 0~64
	sim := 1 - avg/64
	if sim < 0 {
		sim = 0
	}
	return sim
}
