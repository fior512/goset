package utils

import (
	"os"
	"strings"
)

func ReadFileTrim(path string) (result string, err error) {
	var content []byte
	if content, err = os.ReadFile(path); err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}
