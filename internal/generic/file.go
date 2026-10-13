package generic

import (
	"os"
	"strings"
)

func ReadFileTrim(path string) (string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(text)), nil
}
