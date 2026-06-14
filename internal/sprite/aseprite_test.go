package sprite

import (
	"encoding/json"
	"image"
	"testing"
)

func TestBuildAsepriteJSON(t *testing.T) {
	mkFrames := func(n int) []*image.NRGBA {
		out := make([]*image.NRGBA, n)
		for i := range out {
			f := image.NewNRGBA(image.Rect(0, 0, 32, 32))
			for p := 0; p+3 < len(f.Pix); p += 4 {
				f.Pix[p], f.Pix[p+3] = 200, 255
			}
			out[i] = f
		}
		return out
	}
	states := []StateFrames{
		{Spec: StateSpec{Name: "idle", FPS: 8, Loop: true}, Frames: mkFrames(2)},
		{Spec: StateSpec{Name: "attack", FPS: 12, Loop: false}, Frames: mkFrames(3)},
	}
	_, manifest := ComposeAtlas("hero", states, 32, 32)

	data, err := BuildAsepriteJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var sheet struct {
		Frames []struct {
			Filename string `json:"filename"`
			Frame    struct{ X, Y, W, H int }
			Duration int `json:"duration"`
		} `json:"frames"`
		Meta struct {
			FrameTags []struct {
				Name      string `json:"name"`
				From, To  int
				Direction string `json:"direction"`
				Repeat    string `json:"repeat"`
			} `json:"frameTags"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &sheet); err != nil {
		t.Fatal(err)
	}
	if len(sheet.Frames) != 5 {
		t.Fatalf("frames=%d want 5", len(sheet.Frames))
	}
	if len(sheet.Meta.FrameTags) != 2 {
		t.Fatalf("tags=%d want 2", len(sheet.Meta.FrameTags))
	}
	// row 순서 = 정의 순서
	if sheet.Meta.FrameTags[0].Name != "idle" || sheet.Meta.FrameTags[0].From != 0 || sheet.Meta.FrameTags[0].To != 1 {
		t.Errorf("idle tag wrong: %+v", sheet.Meta.FrameTags[0])
	}
	if sheet.Meta.FrameTags[1].Name != "attack" || sheet.Meta.FrameTags[1].From != 2 || sheet.Meta.FrameTags[1].To != 4 {
		t.Errorf("attack tag wrong: %+v", sheet.Meta.FrameTags[1])
	}
	if sheet.Meta.FrameTags[1].Repeat != "1" {
		t.Error("non-loop state must have repeat=1")
	}
	if sheet.Frames[0].Duration != 125 {
		t.Errorf("idle duration=%d want 125", sheet.Frames[0].Duration)
	}
	if sheet.Frames[2].Duration != 83 {
		t.Errorf("attack duration=%d want 83", sheet.Frames[2].Duration)
	}
}

func TestEncodeAPNG(t *testing.T) {
	f := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for p := 0; p+3 < len(f.Pix); p += 4 {
		f.Pix[p], f.Pix[p+3] = 128, 200 // 부분 알파 포함
	}
	data, err := EncodeAPNG([]*image.NRGBA{f, f, f}, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	// PNG 시그니처 + acTL(애니메이션 컨트롤) 청크 존재 확인
	if len(data) < 8 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Fatal("not a PNG")
	}
	if !containsBytes(data, []byte("acTL")) {
		t.Error("missing acTL chunk (not animated)")
	}
}

func containsBytes(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
