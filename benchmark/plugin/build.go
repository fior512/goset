package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func BuildC(name, source string, ldflags ...string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	hash := sha256.Sum256([]byte(source))
	dir := filepath.Join(cacheDir, "goset-bench", name+"-"+hex.EncodeToString(hash[:8]))
	bin := filepath.Join(dir, name)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("%s: cache dir: %w", name, err)
	}
	src := filepath.Join(dir, name+".c")
	if err := os.WriteFile(src, []byte(source), 0o644); err != nil {
		return "", fmt.Errorf("%s: write source: %w", name, err)
	}
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	args := append([]string{"-O2", "-o", bin, src}, ldflags...)
	cmd := exec.Command(cc, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: build: %w", name, err)
	}
	return bin, nil
}
