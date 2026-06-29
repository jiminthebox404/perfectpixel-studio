package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"perfectpixel/internal/gen"
	"perfectpixel/internal/sprite"
)

// runGen은 전체 파이프라인을 실행합니다: 베이스 생성 → 상태별 생성 → 번들 내보내기 → 요약 출력.
func runGen(opt options) error {
	p, provider, model, err := resolveProvider(opt)
	if err != nil {
		return err
	}
	var presets []sprite.PresetInfo
	if !opt.baseOnly && opt.turnaround == 0 {
		presets, err = selectStates(opt)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(opt.out, 0o755); err != nil {
		return err
	}

	logf := func(format string, a ...any) {
		if !opt.quiet && !opt.jsonOut {
			fmt.Printf(format, a...)
		}
	}
	logf("프로바이더: %s · 모델: %s · 스타일: %s · 상태 %d개 · 출력: %s\n",
		provider, model, opt.style, len(presets), opt.out)

	ctx, cancel := context.WithTimeout(context.Background(), opt.timeout)
	defer cancel()

	style := sprite.ResolveStyle(opt.style, "")

	if opt.turnaround > 0 {
		return runTurnaround(ctx, p, opt, style)
	}

	// 1) 베이스 캐릭터 (-base 지정 시 로드, 아니면 생성)
	var baseClean *image.NRGBA
	var baseBytes []byte
	if opt.base != "" {
		logf("base 로드: %s\n", opt.base)
		baseClean, baseBytes, err = loadBaseFromFile(opt.base, opt.style)
		if err != nil {
			return fmt.Errorf("base 로드 실패: %w", err)
		}
	} else {
		logf("베이스 캐릭터 생성 중... ")
		t0 := time.Now()
		baseClean, baseBytes, err = generateBase(ctx, p, opt.desc, opt.style, style)
		if err != nil {
			return fmt.Errorf("베이스 생성 실패: %w", err)
		}
		logf("완료 (%.0fs)\n", time.Since(t0).Seconds())
	}
	savePNG(filepath.Join(opt.out, "base.png"), baseClean)

	// baseonly: 상태/번들 생성을 건너뛰고 base.png만 남긴다.
	if opt.baseOnly {
		if opt.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"provider": provider, "model": model, "style": opt.style,
				"base": filepath.Join(opt.out, "base.png"),
			})
		}
		logf("baseonly 완료 · %s\n", filepath.Join(opt.out, "base.png"))
		return nil
	}

	// 2) 상태별 생성
	var states []sprite.StateFrames
	var rows []resultRow
	for _, kw := range presets {
		spec := sprite.StateSpec{Name: kw.Name, Frames: kw.Frames, FPS: kw.FPS, Loop: kw.Loop, Action: kw.Action}
		ts := time.Now()
		logf("[%s] %s 생성 중... ", kw.Category, kw.Name)
		res := genState(ctx, p, opt, style, spec, [][]byte{baseBytes}, baseClean)
		logf("%d/%d 시도%d 점수%d (%.0fs)\n", res.Found, res.Expected, res.Attempts, res.Score, time.Since(ts).Seconds())
		rows = append(rows, res.row())
		if len(res.frames) > 0 {
			states = append(states, sprite.StateFrames{Spec: spec, Frames: res.frames})
		}
	}

	// 3) 8방향 세트 (선택)
	if strings.TrimSpace(opt.dirset) != "" {
		logf("=== 8방향 세트: %s ===\n", opt.dirset)
		dirStates, dirRows := genDirectionSet(ctx, p, opt, style, opt.dirset, baseBytes, baseClean, logf)
		states = append(states, dirStates...)
		rows = append(rows, dirRows...)
	}

	if len(states) == 0 {
		return fmt.Errorf("생성된 상태가 없어 내보낼 번들이 없습니다")
	}

	// 4) 게임 엔진용 번들 내보내기
	summary, err := exportBundle(opt.out, opt.desc, states, rows)
	if err != nil {
		return fmt.Errorf("내보내기 실패: %w", err)
	}
	summary.Provider = provider
	summary.Model = model
	summary.Style = opt.style

	if opt.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}
	logf("\n번들 완료 · 상태 %d개 · 시트 %dx%d · %s\n",
		summary.Animations, summary.SheetWidth, summary.SheetHeight, opt.out)
	logf("  - sprite-sheet.png / manifest.json / sprite-sheet.json (Aseprite)\n")
	logf("  - frames/<state>/frame-NN.png · gif/<state>.gif · apng/<state>.png\n")
	return nil
}

// runTurnaround은 한 이미지에 N개 뷰(앞→뒤 회전)를 생성하고 ExtractFrames로 분할합니다.
// 같은 캔버스에서 같이 그려 정체성을 공유 → front-bias로 인한 후면 정합 붕괴를 회피.
func runTurnaround(ctx context.Context, p gen.Provider, opt options, style string) error {
	n := opt.turnaround
	if n < 2 {
		n = 4
	}
	if err := os.MkdirAll(opt.out, 0o755); err != nil {
		return err
	}
	logf := func(format string, a ...any) {
		if !opt.quiet && !opt.jsonOut {
			fmt.Printf(format, a...)
		}
	}
	// 뷰 세트 선택: cardinal(정/우/후/좌) 또는 diagonal(¾ 4뷰).
	order := "from left to right: 1) FRONT view, 2) RIGHT-side profile, 3) BACK view seen from directly behind (back of the head and horns, NO face and NO eyes, the occult rune on the upper back), 4) LEFT-side profile"
	names := []string{"front", "right", "back", "left"}
	if opt.turnSet == "diagonal" {
		order = "from left to right, FOUR DISTINCT three-quarter angles (the two back views are mirror images of each other, NOT identical, and both clearly different from a straight-on back view): 1) FRONT-LEFT three-quarter view (rotated about 45°: the face is visible AND we see more of the creature's LEFT side and left shoulder), 2) FRONT-RIGHT three-quarter view (rotated about 45° the other way: face visible AND we see more of the creature's RIGHT side and right shoulder), 3) BACK-LEFT three-quarter view (rotated about 135°: the BACK and the occult rune on the upper back are visible with NO face, AND we see more of the creature's LEFT side and left shoulder), 4) BACK-RIGHT three-quarter view (rotated about 135° the other way: BACK and rune visible with NO face, AND we see more of the creature's RIGHT side and right shoulder)"
		names = []string{"front-left", "front-right", "back-left", "back-right"}
	}

	// 정체성 앵커: -base 지정 시 그 시트를 레퍼런스로 첨부(예: 카디널 시트 → 대각 시트).
	var refs [][]byte
	if opt.base != "" {
		b, err := os.ReadFile(opt.base)
		if err != nil {
			return fmt.Errorf("레퍼런스 시트 로드 실패: %w", err)
		}
		refs = [][]byte{b}
		logf("레퍼런스 시트: %s\n", opt.base)
	}

	identity := "All poses are the very same creature — only the viewing angle changes; identical size, proportions and colors."
	if len(refs) > 0 {
		identity += " The attached reference shows this same creature from other angles — match its identity, colors, horns, eyes and any held object exactly."
	}
	anatomy := "Anatomy is identical across all views: no duplicated, mirrored, extra, or floating limbs; the same limb count and body structure in every pose."
	clauses := identity + " " + anatomy
	if hc := buildHandednessClause(opt.item, opt.hand, opt.turnSet); hc != "" {
		clauses += " " + hc
	}
	prompt := fmt.Sprintf("A character turnaround reference sheet: the SAME single creature shown from %d different angles in ONE horizontal row, evenly spaced with a clear WIDE empty vertical gutter between poses so they are well separated and never touch or overlap, every pose identical in size, proportions, colors and identity. %s. Creature: %s. %s The background is a single solid bright magenta (#FF00FF) chroma-key fill for a clean cutout — no shadows, no gradient, magenta everywhere except the creatures. Bold flat high-contrast pixel art.", n, order, opt.desc, clauses)

	aspect := sprite.AspectForFrames(n)
	logf("턴어라운드 시트 생성 중 (%d뷰 · %s · aspect %s)... ", n, opt.turnSet, aspect)
	raw, err := p.GenerateImage(ctx, prompt, refs, aspect)
	if err != nil {
		return fmt.Errorf("턴어라운드 생성 실패: %w", err)
	}
	img, err := decodeImg(raw)
	if err != nil {
		return err
	}
	savePNG(filepath.Join(opt.out, "turnaround-pre.png"), img)
	clean := sprite.RemoveBackground(img)
	savePNG(filepath.Join(opt.out, "turnaround-raw.png"), clean)

	// 셀 크기: nopixel이면 원본 셀 해상도 유지(다운스케일·양자화로 인한 디테일 손실 방지),
	// 아니면 기존 256 픽셀 셀 + 픽셀화/양자화.
	cellW, cellH, margin := 256, 256, 24
	if opt.noPixel {
		cellW = clean.Rect.Dx()/n + margin*2
		cellH = clean.Rect.Dy() + margin*2
	}
	ext := sprite.ExtractFrames(clean, n, cellW, cellH, margin)
	if !opt.noPixel {
		if ps := sprite.PaletteSizeForStyle(opt.style); ps > 0 && len(ext.Frames) > 0 {
			sprite.LockPalette(sprite.BuildSharedPalette(ext.Frames, ps))
			sprite.PixelPostProcess(ext.Frames, ps)
		}
	}
	for i, f := range ext.Frames {
		nm := fmt.Sprintf("view%d", i)
		if i < len(names) {
			nm = names[i]
		}
		savePNG(filepath.Join(opt.out, "turn-"+nm+".png"), f)
	}
	logf("완료: %d/%d 뷰 분할 (nopixel=%v) → turnaround-raw.png + turn-*.png\n", ext.Found, n, opt.noPixel)
	return nil
}

// buildHandednessClause는 -item/-hand로부터 뷰별 손 위치 지시문을 생성합니다.
// {item}은 자유 슬롯, {hand}=right/left/both가 기하를 결정하므로 프로즈 하드코딩 없이
// 템플릿화됩니다. turnSet(cardinal/diagonal)에 따라 표현을 달리합니다.
func buildHandednessClause(item, hand, turnSet string) string {
	item = strings.TrimSpace(item)
	hand = strings.ToLower(strings.TrimSpace(hand))
	if item == "" || hand == "" {
		return ""
	}
	if hand == "both" {
		return fmt.Sprintf("The creature cradles %s in BOTH hands at the center of its chest — symmetric and the same in every view; fully visible from the front and front-three-quarter views, and partly behind the body from behind.", item)
	}
	if hand != "left" && hand != "right" {
		return ""
	}
	core := fmt.Sprintf("The %s is ALWAYS held in the creature's %s hand and NEVER switches hands; when that hand faces away from the viewer the %s is partly hidden behind the body, never moved to the other hand.", item, hand, item)
	frontSide, backSide := "left", "right"
	nearProfile, farProfile := "RIGHT", "LEFT"
	if hand == "left" {
		frontSide, backSide = "right", "left"
		nearProfile, farProfile = "LEFT", "RIGHT"
	}
	if turnSet == "diagonal" {
		return core + fmt.Sprintf(" In the two front-facing three-quarter views the %s hand is toward the viewer so the %s is visible; in the two back-facing three-quarter views that hand is on the far side so the %s is mostly hidden behind the body.", hand, item, item)
	}
	return core + fmt.Sprintf(" FRONT view: the %s is on the viewer's %s. %s-side view: the %s hand is nearest the viewer, the %s held forward and fully visible. BACK view: the %s is on the viewer's %s (horizontal flip of the front). %s-side view: the %s hand is on the far side, so the %s is hidden behind the body and the near hand is empty.",
		item, frontSide, nearProfile, hand, item, item, backSide, farProfile, hand, item)
}

// genDirectionSet은 5방향 AI 생성 + 3방향 미러링으로 8방향 세트를 만듭니다.
func genDirectionSet(ctx context.Context, p gen.Provider, opt options, style, key string,
	baseBytes []byte, baseClean *image.NRGBA, logf func(string, ...any)) ([]sprite.StateFrames, []resultRow) {

	pre, ok := sprite.PresetByName(key)
	if !ok {
		logf("8방향 세트: 알 수 없는 키워드 %q (건너뜀)\n", key)
		return nil, nil
	}
	var states []sprite.StateFrames
	var rows []resultRow
	frameByDir := map[string][]*image.NRGBA{}
	var southRef []byte

	aiDirs := []string{"south", "east", "north", "south-east", "north-east"}
	if opt.noMirror {
		aiDirs = []string{"south", "east", "north", "south-east", "north-east", "west", "south-west", "north-west"}
	}
	for _, d := range aiDirs {
		spec := sprite.StateSpec{Name: key + "-" + d, Frames: pre.Frames, FPS: pre.FPS, Loop: pre.Loop, Action: pre.Action, Facing: d}
		refs := [][]byte{baseBytes}
		if d != "south" && southRef != nil && !sprite.IsBackFacing(d) {
			refs = append(refs, southRef)
		}
		var bN *image.NRGBA
		if !sprite.IsBackFacing(d) {
			bN = baseClean
		}
		logf("  [%s] 생성 중... ", d)
		res := genState(ctx, p, opt, style, spec, refs, bN)
		logf("%d/%d 점수%d\n", res.Found, res.Expected, res.Score)
		rows = append(rows, res.row())
		if len(res.frames) > 0 {
			states = append(states, sprite.StateFrames{Spec: spec, Frames: res.frames})
			frameByDir[d] = res.frames
			if d == "south" && res.rawClean != nil {
				southRef = pngBytes(res.rawClean)
			}
		}
	}

	if opt.noMirror {
		return states, rows
	}
	// 미러 방향: west<-east, south-west<-south-east, north-west<-north-east
	mirror := map[string]string{"west": "east", "south-west": "south-east", "north-west": "north-east"}
	for dst, src := range mirror {
		srcFrames := frameByDir[src]
		if len(srcFrames) == 0 {
			continue
		}
		var mirrored []*image.NRGBA
		for _, f := range srcFrames {
			mirrored = append(mirrored, sprite.MirrorNRGBA(f))
		}
		spec := sprite.StateSpec{Name: key + "-" + dst, Frames: pre.Frames, FPS: pre.FPS, Loop: pre.Loop, Action: pre.Action, Facing: dst}
		states = append(states, sprite.StateFrames{Spec: spec, Frames: mirrored})
		rows = append(rows, resultRow{Name: spec.Name, Expected: pre.Frames, Found: len(mirrored), Status: "mirrored"})
		logf("  [%s] 미러링(%s) %d프레임\n", dst, src, len(mirrored))
	}
	return states, rows
}
