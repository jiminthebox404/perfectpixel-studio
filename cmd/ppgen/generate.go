package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"
)

// stateResult는 한 상태 생성의 결과 + 품질 측정값입니다.
type stateResult struct {
	Name     string
	Expected int
	Found    int
	Attempts int
	Score    int
	Identity float64
	Motion   float64
	Contact  float64
	Errors   []string
	frames   []*image.NRGBA
	rawClean *image.NRGBA
}

func (r stateResult) ok() bool { return r.Found == r.Expected && len(r.Errors) == 0 }

func (r stateResult) status() string {
	if r.ok() {
		return "ok"
	}
	if r.Found != r.Expected {
		return "frame-mismatch"
	}
	if len(r.Errors) > 0 {
		return "quality-issue"
	}
	return "partial"
}

func (r stateResult) row() resultRow {
	return resultRow{
		Name: r.Name, Expected: r.Expected, Found: r.Found, Attempts: r.Attempts,
		Score: r.Score, Identity: r.Identity, Motion: r.Motion, Contact: r.Contact,
		Status: r.status(), Errors: r.Errors,
	}
}

func decodeImg(raw []byte) (*image.NRGBA, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return sprite.ToNRGBA(img), nil
}

func pngBytes(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func savePNG(path string, img image.Image) {
	if b := pngBytes(img); len(b) > 0 {
		_ = os.WriteFile(path, b, 0o644)
	}
}

// generateBase는 베이스 캐릭터를 생성하고 배경 제거 + 픽셀화한 정리본과 PNG 바이트를 반환합니다.
func generateBase(ctx context.Context, p gen.Provider, desc, styleKey, style string) (*image.NRGBA, []byte, error) {
	raw, err := p.GenerateImage(ctx, sprite.BuildCharacterPrompt(desc, style), nil, "1:1")
	if err != nil {
		return nil, nil, err
	}
	img, err := decodeImg(raw)
	if err != nil {
		return nil, nil, err
	}
	clean := sprite.RemoveBackground(img)
	if paletteSize := sprite.PaletteSizeForStyle(styleKey); paletteSize > 0 {
		// 개체 전역 톤 일관: base에서 팔레트를 만들어 락 → 이후 모든 상태가 같은 팔레트 사용
		sprite.LockPalette(sprite.BuildSharedPalette([]*image.NRGBA{clean}, paletteSize))
		single := []*image.NRGBA{clean}
		sprite.PixelPostProcess(single, paletteSize)
		clean = single[0]
	}
	return clean, pngBytes(clean), nil
}

// loadBaseFromFile은 기존 base.png를 로드해 (생성 대신) 단일 base 앵커로 씁니다.
// 이미 배경 제거·픽셀화된 파일을 가정하고, 팔레트만 락합니다.
func loadBaseFromFile(path, styleKey string) (*image.NRGBA, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	img, err := decodeImg(raw)
	if err != nil {
		return nil, nil, err
	}
	if paletteSize := sprite.PaletteSizeForStyle(styleKey); paletteSize > 0 {
		sprite.LockPalette(sprite.BuildSharedPalette([]*image.NRGBA{img}, paletteSize))
	}
	return img, pngBytes(img), nil
}

// genState는 앱과 동일한 자동 재시도 품질 보정 루프로 한 상태를 생성합니다.
func genState(ctx context.Context, p gen.Provider, opt options, style string,
	spec sprite.StateSpec, refs [][]byte, baseN *image.NRGBA) stateResult {

	expected := spec.Frames
	aspect := sprite.AspectForFrames(expected)
	palette := sprite.PaletteSizeForStyle(opt.style)
	feedback := ""
	maxAttempts := opt.attempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var best stateResult
	bestScore := -1 << 30
	best.Name, best.Expected = spec.Name, expected

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		prompt := sprite.BuildStripPrompt(opt.desc, style, spec, feedback)
		if sprite.IsBackFacing(spec.Facing) {
			prompt += " This is a REAR view: show the BACK of the character (back of the head and horns, back of the body/robe), with NO face and NO eyes visible; the occult rune on the upper back IS visible. The attached reference image shows the FRONT, for identity/colors only — do NOT copy the front, render the back."
		}
		if len(refs) > 1 {
			prompt += "\nMotion reference: the second attached image is the FRONT-view animation strip of this same character performing this exact action. Reproduce the same motion timing and pose phases frame by frame, but viewed from the required facing direction above.\n"
		}
		raw, err := p.GenerateImage(ctx, prompt, refs, aspect)
		if err != nil {
			best.Errors = append(best.Errors, err.Error())
			return best
		}
		nimg, err := decodeImg(raw)
		if err != nil {
			continue
		}
		bgKey := sprite.DetectBackground(nimg)
		clean := sprite.RemoveBackground(nimg)
		cellW, cellH, margin := 256, 256, 24
		if opt.noPixel {
			// 풀디테일: 원본 셀 해상도 유지(다운스케일·양자화 없이)
			cellW = clean.Rect.Dx()/expected + margin*2
			cellH = clean.Rect.Dy() + margin*2
		}
		ext := sprite.ExtractFrames(clean, expected, cellW, cellH, margin)
		insp := sprite.InspectFrames(ext.Frames, bgKey, baseN)
		if !opt.noPixel {
			sprite.PixelPostProcess(ext.Frames, palette)
		}

		cand := stateResult{
			Name: spec.Name, Expected: expected, Found: ext.Found, Attempts: attempt,
			Motion: sprite.MotionPresence(ext.Frames), frames: ext.Frames, rawClean: clean,
		}
		cand.Errors = append(cand.Errors, insp.Errors...)

		if cand.ok() {
			qm := sprite.ScoreFrames(ext.Frames)
			cand.Score = int(qm.Overall * 100)
			cand.Identity = qm.Identity
			cand.Motion = qm.Motion
			cand.Contact = qm.Contact
			return cand
		}
		score := cand.Found*100 - len(cand.Errors)*10
		if score > bestScore {
			best, bestScore = cand, score
		}

		var fixes []string
		if cand.Found != expected {
			fixes = append(fixes, fmt.Sprintf(
				"IMPORTANT CORRECTION: the last attempt read as %d poses but EXACTLY %d are required. Redraw as %d equal columns, one clearly separated pose per column, each ringed by a clean magenta gutter.",
				cand.Found, expected, expected))
		}
		fixes = append(fixes, insp.RetryHints...)
		feedback = strings.Join(fixes, "\n")
	}
	if len(best.frames) > 0 {
		qm := sprite.ScoreFrames(best.frames)
		best.Score = int(qm.Overall * 100)
		best.Identity = qm.Identity
		best.Motion = qm.Motion
		best.Contact = qm.Contact
	}
	return best
}
