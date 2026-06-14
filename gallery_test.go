package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// 폴더 나열: 이미지만 필터링되고 이름순으로 정렬되는지 검증
func TestListFolderImages(t *testing.T) {
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	if err := writePNG(filepath.Join(dir, "b.png"), img); err != nil {
		t.Fatal(err)
	}
	if err := writePNG(filepath.Join(dir, "a.png"), img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	items, err := app.ListFolderImages(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("이미지 2개를 기대했지만 %d개 반환", len(items))
	}
	if items[0].Name != "a.png" || items[1].Name != "b.png" {
		t.Fatalf("이름순 정렬 실패: %s, %s", items[0].Name, items[1].Name)
	}
	if items[0].Size <= 0 || items[0].ModTime <= 0 {
		t.Fatalf("메타데이터 누락: %+v", items[0])
	}
}

// 썸네일: maxDim 안으로 비율 유지 다운스케일되는지 검증
func TestLoadImageThumbDownscale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.png")
	if err := writePNG(path, image.NewNRGBA(image.Rect(0, 0, 600, 300))); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	url, err := app.LoadImageThumb(path, 200)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := decodeDataURL(url)
	if err != nil {
		t.Fatal(err)
	}
	thumb, err := decodeImage(raw)
	if err != nil {
		t.Fatal(err)
	}
	b := thumb.Bounds()
	if b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("200x100 썸네일을 기대했지만 %dx%d", b.Dx(), b.Dy())
	}
}

// 작은 이미지는 재인코딩 없이 원본 dataURL을 그대로 반환
func TestLoadImageThumbSmallPassthrough(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.png")
	if err := writePNG(path, image.NewNRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	url, err := app.LoadImageThumb(path, 200)
	if err != nil {
		t.Fatal(err)
	}
	full, err := app.LoadImageFull(path)
	if err != nil {
		t.Fatal(err)
	}
	if url != full {
		t.Fatal("작은 이미지는 원본 dataURL을 그대로 반환해야 함")
	}
}

// 갤러리 디렉토리 밖의 파일 삭제는 거부되어야 함
func TestDeleteGalleryImageGuard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := writePNG(path, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	if err := app.DeleteGalleryImage(path); err == nil {
		t.Fatal("갤러리 외부 파일 삭제가 허용되면 안 됨")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("거부된 삭제 요청으로 파일이 사라짐")
	}
}
