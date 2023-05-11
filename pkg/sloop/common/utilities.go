package common

import (
	"fmt"
	"path"
	"strings"
)

func BoolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func ParseKey(key string) (error, []string) {
	parts := strings.Split(key, "/")
	if len(parts) != 7 {
		return fmt.Errorf("key should have 6 parts: %v", key), parts
	}
	if parts[0] != "" {
		return fmt.Errorf("key should start with /: %v", key), parts
	}

	return nil, parts
}

func Contains(stringList []string, elem string) bool {
	for _, str := range stringList {
		if str == elem {
			return true
		}
	}
	return false
}

func GetFilePath(filePath string, fileName string) string {
	return path.Join(filePath, fileName)
}

func Max(x int, y int) int {
	if x < y {
		return y
	}
	return x
}

func Truncate(text string, width int, delimiter ...string) (string, error) {
	d := "..."
	if len(delimiter) > 0 {
		d = delimiter[0]
	}
	d_len := len(d)
	if width < 0 {
		return "", fmt.Errorf("invalid width")
	}
	if len(text) <= width {
		return text, nil
	}
	r := []rune(text)
	truncated := r[:(Max(width, d_len) - d_len)]
	return string(truncated) + d, nil
}
