package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"

	"perfectpixel/internal/sprite"
)

// loadPNG는 PNG를 NRGBA로 읽는다.
func loadPNG(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return sprite.ToNRGBA(im), nil
}

// countFrames는 디렉토리의 frame-*.png 개수를 센다(기대 프레임 수).
func countFrames(dir string) int {
	m, _ := filepath.Glob(filepath.Join(dir, "frame-*.png"))
	return len(m)
}

type scanRow struct {
	path      string
	n         int
	found     int
	balWo     float64 // equal-split 콘텐츠 불균형
	balWi     float64
	cenWo     float64 // equal-split(bbox center) 질량중심 표준편차
	cenWi     float64 // ExtractFrames(centroid) 질량중심 표준편차
	segGain   float64 // 분할 이득 = balWo/balWi
	cenGain   float64 // 정렬 이득 = cenWo - cenWi
	cross     int     // 균등분할선이 캐릭터를 가로지른 개수
}

// scanSamples는 sample/*/*/_strip.png 전부에 대해 적용 전/후 메트릭을 계산해
// 분할/정렬 대비가 큰 스트립을 찾는다(실제 AI 데이터에서 데모 케이스 선정).
func scanSamples() {
	strips, _ := filepath.Glob("sample/*/*/_strip.png")
	var rows []scanRow
	for _, p := range strips {
		dir := filepath.Dir(p)
		n := countFrames(dir)
		if n < 2 {
			continue
		}
		strip, err := loadPNG(p)
		if err != nil {
			continue
		}
		wo := equalSplitExtract(strip, n, cell, cell, 16)
		ext := sprite.ExtractFrames(strip, n, cell, cell, 16)
		_, _, rWo := frameBalance(wo)
		_, _, rWi := frameBalance(ext.Frames)
		cWo := centroidSpread(wo)
		cWi := centroidSpread(ext.Frames)
		seg := rWo
		if rWi > 0 {
			seg = rWo / rWi
		}
		cross, _ := crossingLines(strip, n)
		rows = append(rows, scanRow{p, n, ext.Found, rWo, rWi, cWo, cWi, seg, cWo - cWi, cross})
	}
	fmt.Printf("스캔 %d개 스트립\n", len(rows))

	sort.Slice(rows, func(i, j int) bool { return rows[i].cross > rows[j].cross })
	fmt.Println("\n== 균등분할선이 캐릭터를 가로지름 상위 (cross/n-1) ==")
	for i := 0; i < len(rows) && i < 8; i++ {
		r := rows[i]
		fmt.Printf("  %-40s n=%d  cross=%d/%d  cenGain=%.1fpx\n", rel(r.path), r.n, r.cross, r.n-1, r.cenGain)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].segGain > rows[j].segGain })
	fmt.Println("\n== 분할 대비 상위 (balWo/balWi 클수록 equal-split 실패) ==")
	for i := 0; i < len(rows) && i < 6; i++ {
		r := rows[i]
		fmt.Printf("  %-40s n=%d found=%d  balWo=%.1f balWi=%.1f gain=%.1fx\n",
			rel(r.path), r.n, r.found, r.balWo, r.balWi, r.segGain)
	}

	fmt.Println("\n== 포즈 병합/누락 (found != n) 또는 balWo 최대 ==")
	sort.Slice(rows, func(i, j int) bool { return rows[i].balWo > rows[j].balWo })
	for i := 0; i < len(rows) && i < 8; i++ {
		r := rows[i]
		flag := ""
		if r.found != r.n {
			flag = "  <-- found!=n"
		}
		fmt.Printf("  %-40s n=%d found=%d balWo=%.2f%s\n", rel(r.path), r.n, r.found, r.balWo, flag)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].cenGain > rows[j].cenGain })
	fmt.Println("\n== 정렬 대비 상위 (cenWo - cenWi 클수록 centroid 이득) ==")
	for i := 0; i < len(rows) && i < 6; i++ {
		r := rows[i]
		fmt.Printf("  %-40s n=%d  cenWo=%.1f cenWi=%.1f gain=%.1fpx\n",
			rel(r.path), r.n, r.cenWo, r.cenWi, r.cenGain)
	}
}

func rel(p string) string {
	parts := filepath.SplitList(p)
	_ = parts
	d := filepath.Dir(p)
	return filepath.Base(filepath.Dir(d)) + "/" + filepath.Base(d)
}
