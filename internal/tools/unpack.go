package tools

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// EnsureUnpacked checks if a binary at path exists. If not, it looks for
// path+".gz" and decompresses it to path with executable permissions.
// If the binary already exists, it is a no-op.
func EnsureUnpacked(path string) error {
	if _, err := os.Stat(path); err == nil {
		// 이미 존재하면 스킵
		return nil
	}

	gzPath := path + ".gz"
	if _, err := os.Stat(gzPath); os.IsNotExist(err) {
		return fmt.Errorf("neither %s nor %s found", path, gzPath)
	}

	gz, err := os.Open(gzPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", gzPath, err)
	}
	defer gz.Close()

	gr, err := gzip.NewReader(gz)
	if err != nil {
		return fmt.Errorf("gzip reader %s: %w", gzPath, err)
	}
	defer gr.Close()

	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, gr); err != nil {
		os.Remove(path) // 실패 시 불완전한 파일 정리
		return fmt.Errorf("decompress %s: %w", gzPath, err)
	}

	return nil
}
