package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A loose copy beside the app wins over the bundled one, so a model can be
// swapped without a rebuild.
func TestFindAssetPrefersTheLooseCopy(t *testing.T) {
	p, err := findAsset("ppocrv5_dict.txt")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(filepath.Join("ocr", "ppocrv5_dict.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if same, _ := filepath.Abs(p); same != want {
		t.Fatalf("findAsset chose %q, want the copy in the repo %q", p, want)
	}
}

// Without a loose copy the file is unpacked from the executable into the
// cache folder, and a second call reuses it.
func TestBundledAssetUnpacksOnce(t *testing.T) {
	dir := t.TempDir()
	old := assetDir
	assetDir = dir
	defer func() { assetDir = old }()

	p, err := unpackAsset("ppocrv5_dict.txt")
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(dir, "ppocrv5_dict.txt") {
		t.Fatalf("unpacked to %q", p)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want, err := bundled.ReadFile("ocr/ppocrv5_dict.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) || string(got[:64]) != string(want[:64]) {
		t.Fatalf("unpacked %d bytes, want %d", len(got), len(want))
	}

	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := unpackAsset("ppocrv5_dict.txt")
	if err != nil {
		t.Fatal(err)
	}
	st2, err := os.Stat(again)
	if err != nil {
		t.Fatal(err)
	}
	if !st2.ModTime().Equal(st.ModTime()) {
		t.Fatal("the second call rewrote the file instead of reusing it")
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Fatalf("the cache folder holds %d files (%v), want just the asset", len(entries), err)
	}
}

// Every file the app reads at run time is bundled, so nothing has to be
// shipped beside the executable.
func TestBundleHoldsEveryAsset(t *testing.T) {
	for _, name := range []string{
		"onnxruntime.dll",
		"PP-OCRv5_mobile_det_infer.onnx",
		"PP-OCRv5_mobile_rec_infer.onnx",
		"ppocrv5_dict.txt",
		"ecdict-common.json",
	} {
		st, err := bundled.Open("ocr/" + name)
		if err != nil {
			t.Errorf("%s is not bundled: %v", name, err)
			continue
		}
		fi, err := st.Stat()
		st.Close()
		if err != nil || fi.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}
