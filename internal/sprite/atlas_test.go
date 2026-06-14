package sprite

import (
	"image"
	"testing"
)

// TestComposeManifestV2는 합성 매니페스트가 v2 스키마(피벗/트림/지속시간)를
// 올바르게 채우는지 검증합니다.
func TestComposeManifestV2(t *testing.T) {
	mk := func() *image.NRGBA {
		f := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		fillBox(f, 20, 30, 43, 60, 200, 100, 50) // 하단 중앙 콘텐츠
		return f
	}
	states := []StateFrames{
		{Spec: StateSpec{Name: "idle", FPS: 8, Loop: true}, Frames: []*image.NRGBA{mk(), mk()}},
	}
	_, m := ComposeAtlas("hero", states, 64, 64)

	if m.Version != 2 || m.Schema == "" || m.Generator == "" {
		t.Fatalf("v2 메타 누락: version=%d schema=%q generator=%q", m.Version, m.Schema, m.Generator)
	}
	a, ok := m.Animations["idle"]
	if !ok {
		t.Fatal("idle 애니메이션 누락")
	}
	if a.DurationMs != 125 { // 1000/8
		t.Fatalf("durationMs 오류: %d", a.DurationMs)
	}
	if len(a.Trims) != 2 {
		t.Fatalf("trim 수 오류: %d", len(a.Trims))
	}
	if a.Trims[0].W <= 0 || a.Trims[0].H <= 0 {
		t.Fatalf("trim bbox 비정상: %+v", a.Trims[0])
	}
	// 피벗은 셀 가로중심 + 콘텐츠 최하단
	if a.Pivot.X != 32 {
		t.Fatalf("pivot.X 오류: %d", a.Pivot.X)
	}
	if a.Pivot.Y < 55 || a.Pivot.Y > 64 {
		t.Fatalf("pivot.Y(발 앵커) 오류: %d", a.Pivot.Y)
	}
}
