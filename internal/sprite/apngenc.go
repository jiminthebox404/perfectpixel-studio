package sprite

import (
	"bytes"
	"image"

	"github.com/kettek/apng"
)

// EncodeAPNG는 프레임들을 APNG로 인코딩합니다.
// GIF와 달리 8-bit 풀 알파를 지원해 가장자리가 깨지지 않습니다.
func EncodeAPNG(frames []*image.NRGBA, fps int, loop bool) ([]byte, error) {
	if fps <= 0 {
		fps = 8
	}
	a := apng.APNG{}
	if !loop {
		a.LoopCount = 1
	}
	for _, f := range frames {
		a.Frames = append(a.Frames, apng.Frame{
			Image:            f,
			DelayNumerator:   1,
			DelayDenominator: uint16(fps),
		})
	}
	var buf bytes.Buffer
	if err := apng.Encode(&buf, a); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
