// The data files the app needs at run time: the onnxruntime library, the
// PP-OCRv5 models, their dictionary and the offline word list. They are
// compiled into the executable and unpacked to a cache folder the first
// time they are used, so the app runs from any directory on its own. A
// copy beside the executable still wins, for swapping a model without a
// rebuild.

package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//go:embed ocr/onnxruntime.dll ocr/PP-OCRv5_mobile_det_infer.onnx ocr/PP-OCRv5_mobile_rec_infer.onnx ocr/ppocrv5_dict.txt ocr/ecdict-common.json
var bundled embed.FS

// assetDir is where the bundled files are unpacked. Tests point it at a
// temporary folder.
var assetDir = defaultAssetDir()

func defaultAssetDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, appSlug, "assets")
}

var unpackMu sync.Mutex

// findAsset returns the path a data file can be read from: a copy shipped
// beside the executable or the working directory, else the bundled copy,
// unpacked to the cache folder on first use.
func findAsset(name string) (string, error) {
	if p := assetBeside(name); p != "" {
		return p, nil
	}
	return unpackAsset(name)
}

// assetBeside looks for a loose copy of a data file: the ocr/ folder next
// to the executable or the working directory, then the same folders
// without the ocr prefix, then the folders one and two levels above the
// executable, which is where a development build finds the sources.
func assetBeside(name string) string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		dirs = append(dirs,
			filepath.Join(d, "ocr"), d,
			filepath.Join(filepath.Dir(d), "ocr"), filepath.Dir(d),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(wd, "ocr"), wd)
	}
	seen := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		if seen[d] {
			continue
		}
		seen[d] = true
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// unpackAsset writes the bundled copy of a data file into the cache folder
// and returns its path. A file already there with the right size is reused.
func unpackAsset(name string) (string, error) {
	data, err := bundled.ReadFile("ocr/" + name)
	if err != nil {
		return "", fmt.Errorf("缺少 %s", name)
	}
	unpackMu.Lock()
	defer unpackMu.Unlock()
	dst := filepath.Join(assetDir, name)
	if st, err := os.Stat(dst); err == nil && st.Size() == int64(len(data)) {
		return dst, nil
	}
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return "", fmt.Errorf("解包 %s 失败: %w", name, err)
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("解包 %s 失败: %w", name, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("解包 %s 失败: %w", name, err)
	}
	return dst, nil
}
