package sprite

import (
	"image"
	"testing"
)

func TestPresetsCatalog(t *testing.T) {
	if len(Presets) != 100 {
		t.Fatalf("프리셋 개수 = %d, 기대값 100", len(Presets))
	}

	seen := map[string]bool{}
	for _, p := range Presets {
		if p.Name == "" || p.Label == "" || p.Category == "" {
			t.Errorf("프리셋 필드 누락: %+v", p)
		}
		if seen[p.Name] {
			t.Errorf("중복 프리셋 이름: %q", p.Name)
		}
		seen[p.Name] = true

		// 모션 힌트는 자연스러운 애니메이션 품질의 핵심이므로 모든 키워드에 필수
		if len(p.Hint) < 20 {
			t.Errorf("프리셋 %q의 모션 힌트가 비었거나 너무 짧음", p.Name)
		}
		if p.Frames < 1 || p.Frames > 10 {
			t.Errorf("프리셋 %q의 프레임 수 %d는 1~10 범위 밖", p.Name, p.Frames)
		}
		if p.FPS < 1 || p.FPS > 30 {
			t.Errorf("프리셋 %q의 FPS %d는 1~30 범위 밖", p.Name, p.FPS)
		}
		if p.Action == "" {
			t.Errorf("프리셋 %q의 동작 설명 누락", p.Name)
		}
	}
}

func TestMotionHintFromCatalog(t *testing.T) {
	// 카탈로그의 모든 이름이 MotionHint로 조회되어야 함
	for _, p := range Presets {
		if MotionHint(p.Name) == "" {
			t.Errorf("MotionHint(%q)가 비었음", p.Name)
		}
	}
	// 대소문자/공백 정규화 확인
	if MotionHint("  IDLE  ") == "" {
		t.Error("MotionHint 정규화 실패")
	}
	// 미등록 이름은 빈 문자열
	if MotionHint("nonexistent-state-xyz") != "" {
		t.Error("미등록 상태는 빈 힌트를 반환해야 함")
	}
}

func TestMotionHintStripsDirectionSuffix(t *testing.T) {
	// 8방향 세트 상태명은 방향 접미사가 붙어도 베이스 힌트를 찾아야 함
	base := MotionHint("attack")
	if base == "" {
		t.Fatal("attack 힌트가 비었음")
	}
	cases := []string{
		"attack-south", "attack-north", "attack-east", "attack-west",
		"attack-south-east", "attack-north-east", "attack-south-west", "attack-north-west",
	}
	for _, name := range cases {
		if got := MotionHint(name); got != base {
			t.Errorf("MotionHint(%q) = %q, attack 베이스 힌트와 달라야 하지 않음", name, truncate(got))
		}
	}
	// 복합 방향이 단일 방향으로 잘못 잘리지 않는지 확인
	if stripDirectionSuffix("attack-south-east") != "attack" {
		t.Errorf("복합 방향 접미사 제거 실패: %q", stripDirectionSuffix("attack-south-east"))
	}
}

func TestMotionPresence(t *testing.T) {
	mk := func(fill uint8) *image.NRGBA {
		im := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		for i := 0; i < len(im.Pix); i += 4 {
			im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = fill, fill, fill, 255
		}
		return im
	}
	// 동일 프레임 2장 → 움직임 0
	if m := MotionPresence([]*image.NRGBA{mk(100), mk(100)}); m != 0 {
		t.Errorf("동일 프레임 움직임 = %v, 0 기대", m)
	}
	// 흑→백 완전 변화 → 1에 근접
	if m := MotionPresence([]*image.NRGBA{mk(0), mk(255)}); m < 0.7 {
		t.Errorf("큰 변화 움직임 = %v, 높아야 함", m)
	}
	// 프레임 1장 → 0
	if m := MotionPresence([]*image.NRGBA{mk(50)}); m != 0 {
		t.Errorf("단일 프레임 = %v, 0 기대", m)
	}
}

func truncate(s string) string {
	if len(s) > 20 {
		return s[:20] + "..."
	}
	return s
}

func TestListPresetsHidesHint(t *testing.T) {
	out := ListPresets()
	if len(out) != len(Presets) {
		t.Fatalf("ListPresets 길이 불일치")
	}
	// 반환 슬라이스 수정이 원본에 영향 주지 않아야 함 (복사본)
	if len(out) > 0 {
		out[0].Label = "MUTATED"
		if Presets[0].Label == "MUTATED" {
			t.Error("ListPresets가 원본 카탈로그를 노출함 (복사본이어야 함)")
		}
	}
}
