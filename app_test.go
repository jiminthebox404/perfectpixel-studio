package main

import (
	"testing"
)

// TestSessionRoundTrip은 세션 저장 → 복원 → 삭제 흐름을 검증합니다.
// HOME을 임시 디렉토리로 격리해 실제 사용자 세션을 건드리지 않습니다.
func TestSessionRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := NewApp()

	// 저장 전에는 빈 문자열
	if got := a.LoadSession(); got != "" {
		t.Fatalf("초기 세션이 비어있지 않음: %q", got)
	}

	payload := `{"v":1,"character":{"name":"hero"},"cellSize":256,"states":[]}`
	if err := a.SaveSession(payload); err != nil {
		t.Fatalf("세션 저장 실패: %v", err)
	}
	if got := a.LoadSession(); got != payload {
		t.Fatalf("세션 복원 불일치: %q", got)
	}

	// 덮어쓰기
	payload2 := `{"v":1,"character":{"name":"slime"},"cellSize":128,"states":[]}`
	if err := a.SaveSession(payload2); err != nil {
		t.Fatalf("세션 덮어쓰기 실패: %v", err)
	}
	if got := a.LoadSession(); got != payload2 {
		t.Fatalf("덮어쓴 세션 불일치: %q", got)
	}

	// 삭제 후 빈 문자열, 중복 삭제도 에러 없어야 함
	if err := a.ClearSession(); err != nil {
		t.Fatalf("세션 삭제 실패: %v", err)
	}
	if got := a.LoadSession(); got != "" {
		t.Fatalf("삭제 후 세션이 남아있음: %q", got)
	}
	if err := a.ClearSession(); err != nil {
		t.Fatalf("중복 삭제 에러: %v", err)
	}
}
