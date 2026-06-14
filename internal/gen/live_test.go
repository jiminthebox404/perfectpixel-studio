package gen

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"testing"
	"time"
)

// TestFalLive는 실제 fal.ai API를 호출하는 스모크 테스트입니다.
// PP_LIVE_TEST=1 과 FAL_KEY 가 설정된 경우에만 실행됩니다 (크레딧 소모 주의).
func TestFalLive(t *testing.T) {
	if os.Getenv("PP_LIVE_TEST") != "1" {
		t.Skip("PP_LIVE_TEST=1 설정 시에만 실행")
	}
	key := os.Getenv("FAL_KEY")
	if key == "" {
		t.Skip("FAL_KEY 미설정")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := NewFal(key, "")
	data, err := c.GenerateImage(ctx,
		"A single small red circle centered on a solid magenta (#FF00FF) background. Flat colors, no gradients.",
		nil, "1:1")
	if err != nil {
		t.Fatalf("fal 이미지 생성 실패: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("PNG 디코딩 실패 (len=%d): %v", len(data), err)
	}
	b := img.Bounds()
	if b.Dx() < 64 || b.Dy() < 64 {
		t.Fatalf("이미지 크기가 비정상적으로 작음: %v", b)
	}
	t.Logf("fal 생성 성공: %dx%d (%d bytes)", b.Dx(), b.Dy(), len(data))

	// 참조 이미지 → /edit 엔드포인트 경로 검증 (실제 스프라이트 스트립 생성 경로)
	data2, err := c.GenerateImage(ctx,
		"Move the red circle to the left edge. Keep the solid magenta (#FF00FF) background.",
		[][]byte{data}, "1:1")
	if err != nil {
		t.Fatalf("fal edit 생성 실패: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(data2)); err != nil {
		t.Fatalf("edit PNG 디코딩 실패 (len=%d): %v", len(data2), err)
	}
	t.Logf("fal edit 성공: %d bytes", len(data2))
}
