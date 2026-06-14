package sprite

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Aseprite 호환 스프라이트시트 JSON (array form).
// Phaser/Pixi/Unity/Godot의 Aseprite 임포터가 그대로 읽을 수 있는 표준 교환 포맷입니다.

type aseRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type aseSize struct {
	W int `json:"w"`
	H int `json:"h"`
}

type aseFrame struct {
	Filename         string  `json:"filename"`
	Frame            aseRect `json:"frame"`
	Rotated          bool    `json:"rotated"`
	Trimmed          bool    `json:"trimmed"`
	SpriteSourceSize aseRect `json:"spriteSourceSize"`
	SourceSize       aseSize `json:"sourceSize"`
	Duration         int     `json:"duration"` // ms
}

type aseFrameTag struct {
	Name      string `json:"name"`
	From      int    `json:"from"`
	To        int    `json:"to"`
	Direction string `json:"direction"`
	Repeat    string `json:"repeat,omitempty"` // "1" = 1회 재생 (Aseprite 1.3+)
}

type aseMeta struct {
	App       string        `json:"app"`
	Version   string        `json:"version"`
	Image     string        `json:"image"`
	Format    string        `json:"format"`
	Size      aseSize       `json:"size"`
	Scale     string        `json:"scale"`
	FrameTags []aseFrameTag `json:"frameTags"`
}

type aseSheet struct {
	Frames []aseFrame `json:"frames"`
	Meta   aseMeta    `json:"meta"`
}

// BuildAsepriteJSON은 매니페스트를 Aseprite 호환 시트 JSON으로 변환합니다.
func BuildAsepriteJSON(m Manifest) ([]byte, error) {
	// 행(row) 순서대로 상태 정렬 → 프레임 인덱스가 시트 배치와 일치
	type namedAnim struct {
		name string
		anim AnimationEntry
	}
	anims := make([]namedAnim, 0, len(m.Animations))
	for name, a := range m.Animations {
		anims = append(anims, namedAnim{name, a})
	}
	sort.Slice(anims, func(i, j int) bool { return anims[i].anim.Row < anims[j].anim.Row })

	sheet := aseSheet{
		Meta: aseMeta{
			App:       "perfectpixel",
			Version:   "1.0",
			Image:     m.Sheet.Image,
			Format:    "RGBA8888",
			Size:      aseSize{W: m.Sheet.Width, H: m.Sheet.Height},
			Scale:     "1",
			FrameTags: []aseFrameTag{},
		},
	}

	idx := 0
	for _, na := range anims {
		fps := na.anim.FPS
		if fps <= 0 {
			fps = 8
		}
		duration := 1000 / fps
		from := idx
		for fi, r := range na.anim.Rects {
			sheet.Frames = append(sheet.Frames, aseFrame{
				Filename:         fmt.Sprintf("%s %d", na.name, fi),
				Frame:            aseRect{X: r.X, Y: r.Y, W: r.W, H: r.H},
				SpriteSourceSize: aseRect{X: 0, Y: 0, W: r.W, H: r.H},
				SourceSize:       aseSize{W: r.W, H: r.H},
				Duration:         duration,
			})
			idx++
		}
		tag := aseFrameTag{Name: na.name, From: from, To: idx - 1, Direction: "forward"}
		if !na.anim.Loop {
			tag.Repeat = "1"
		}
		sheet.Meta.FrameTags = append(sheet.Meta.FrameTags, tag)
	}

	return json.MarshalIndent(sheet, "", "  ")
}
