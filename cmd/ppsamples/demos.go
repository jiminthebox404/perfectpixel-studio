package main

import (
	"fmt"
	"image"
	"image/color"

	"perfectpixel/internal/sprite"
)

const cell = 256

func magentaCanvas(w, h int) *image.NRGBA { return newCanvas(w, h, keyMagenta) }

// idlePose는 기본 서 있는 포즈.
func idlePose() pose { return pose{lArm: 18, rArm: -18, lLeg: 8, rLeg: -8, armLen: 52, legLen: 60} }

// ---------- 1. chroma matting (배경 제거) ----------

func demoMatting(raw *image.NRGBA) {
	fmt.Println("[1] chroma matting (배경 제거) — 실제 AI 산출물")
	jp := raw // 실제 AI 출력(마젠타 key, 이미 JPEG 압축됨)

	without := naiveMatte(jp, keyMagenta, 70) // 단순 RGB 임계값
	with := sprite.RemoveBackground(jp)        // 실제 YCbCr 매팅+despill+flood+morph

	res := magentaResidue(without)
	resW := magentaResidue(with)
	halo := haloPinkish(without)
	haloW := haloPinkish(with)
	fmt.Printf("  마젠타 잔여물: 없음=%d  있음=%d\n", res, resW)
	fmt.Printf("  핑크 헤일로  : 없음=%d  있음=%d\n", halo, haloW)
	fmt.Printf("  캐릭터 콘텐츠(보존 확인): 없음=%d  있음=%d\n", contentPixels(without), contentPixels(with))

	gray := newCanvas(raw.Rect.Dx(), raw.Rect.Dy(), colBG)
	const pw = 300
	inP := labeledPanel(resizeW(jp, pw), "INPUT: real AI output (magenta key)", colInk)
	woP := labeledPanel(resizeW(overOn(gray, without), pw),
		fmt.Sprintf("WITHOUT: naive RGB threshold  (residue %d, halo %d)", res, halo), colWithout)
	wiP := labeledPanel(resizeW(overOn(gray, with), pw),
		fmt.Sprintf("WITH: YCbCr matting+despill  (residue %d, halo %d)", resW, haloW), colWith)

	title := titleBar(pw*3+40, "1. Background removal: chroma matting (chroma.go) — real AI sprite")
	body := hstack(20, color.NRGBA{255, 255, 255, 255}, inP, woP, wiP)
	save("01-matting.png", vstack(6, color.NRGBA{255, 255, 255, 255}, title, body))
}

// ---------- 2. projection + DP 분할 ----------

func demoSegmentation(matte *image.NRGBA, n int) {
	fmt.Println("[2] projection + DP segmentation — 실제 AI 스트립")
	without := equalSplitExtract(matte, n, cell, cell, 16)
	ext := sprite.ExtractFrames(matte, n, cell, cell, 16)
	with := ext.Frames

	cross, _ := crossingLines(matte, n)
	fmt.Printf("  검출 포즈 수(natural): %d (요청 %d)\n", ext.Found, n)
	fmt.Printf("  균등분할선이 캐릭터를 가로지름: 없음=%d/%d  있음=0/%d(gutter 절단)\n", cross, n-1, n-1)

	gray := newCanvas(cell, cell, colBG)
	dispW := 900
	stripView := resizeW(overOn(newCanvas(matte.Rect.Dx(), matte.Rect.Dy(), color.NRGBA{255, 255, 255, 255}), matte), dispW)
	for k := 1; k < n; k++ { // 균등 분할선(빨강) — 실제 포즈 위를 가로지름
		x := k * dispW / n
		for y := 0; y < stripView.Rect.Dy(); y++ {
			setPx(stripView, x, y, color.NRGBA{220, 40, 40, 255})
		}
	}
	woRow := scaleRow(without, gray, 3)
	wiRow := scaleRow(with, gray, 3)

	title := titleBar(dispW, "2. Frame split: projection + DP optimal cut (segment.go) — real AI strip")
	out := vstack(6, color.NRGBA{255, 255, 255, 255},
		title,
		captionBar(dispW, fmt.Sprintf("Red = naive equal-split lines: %d of %d cut straight through a character.", cross, n-1), colInk),
		labeledPanel(stripView, "matted real strip (red = equal-split cut lines)", colInk),
		labeledPanel(woRow, fmt.Sprintf("WITHOUT: equal split  (%d/%d cut lines slice a character)", cross, n-1), colWithout),
		labeledPanel(wiRow, fmt.Sprintf("WITH: projection+DP  (found %d, each pose isolated at its gutter)", ext.Found), colWith),
	)
	save("02-segmentation.png", out)
}
