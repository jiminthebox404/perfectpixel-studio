// Command resplit은 이미 매트(투명 배경) 처리된 턴어라운드 raw 시트를 받아
// 현재 ExtractFrames 로직으로 다시 분할만 합니다. 생성 변동 없이 분할기 변경을
// 같은 입력으로 검증하기 위한 헤드리스 도구입니다.
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"

	"perfectpixel/internal/sprite"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: resplit <raw.png> <n> <outdir>")
		os.Exit(1)
	}
	rawPath := os.Args[1]
	n, err := strconv.Atoi(os.Args[2])
	if err != nil || n < 1 {
		fmt.Fprintln(os.Stderr, "n은 양의 정수여야 합니다")
		os.Exit(1)
	}
	out := os.Args[3]

	f, err := os.Open(rawPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "raw 열기 실패: %v\n", err)
		os.Exit(1)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "raw 디코드 실패: %v\n", err)
		os.Exit(1)
	}

	clean := sprite.ToNRGBA(img)
	const margin = 24
	cellW := clean.Rect.Dx()/n + margin*2
	cellH := clean.Rect.Dy() + margin*2
	ext := sprite.ExtractFrames(clean, n, cellW, cellH, margin)

	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "출력 디렉토리 생성 실패: %v\n", err)
		os.Exit(1)
	}
	names := []string{"front", "right", "back", "left"}
	for i, fr := range ext.Frames {
		nm := fmt.Sprintf("view%d", i)
		if i < len(names) {
			nm = names[i]
		}
		of, err := os.Create(filepath.Join(out, "resplit-"+nm+".png"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "프레임 저장 실패: %v\n", err)
			os.Exit(1)
		}
		_ = png.Encode(of, fr)
		of.Close()
	}
	fmt.Printf("resplit %d/%d 뷰 → %s (cell %dx%d)\n", ext.Found, n, out, cellW, cellH)
}
