package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func writeToFile(filePath, content string) error {
	fs, err := os.Create(filePath)
	defer fs.Close()
	if err != nil {
		return err
	}
	fs.WriteString(content)
	return nil
}

// expand ~ to user home directory (platform independent)
func expandPath(p string) string {
	expandedPath := os.ExpandEnv(p)
	if strings.HasPrefix(p, "~") {
		pos := strings.Index(expandedPath, "/")
		HomeDir := ""
		if runtime.GOOS == "windows" {
			HomeDir = os.Getenv("USERPROFILE")
		} else {
			HomeDir = os.Getenv("HOME")
		}
		p = filepath.Join(HomeDir, expandedPath[pos:len(expandedPath)])
	}
	p, _ = filepath.Abs(p)
	return p
}

func fileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

func parseTime(formatted string) (time.Time, error) {
	var layouts = [...]string{
		"20060102",
		"2006/01/02",
		"2006/1/2",
		"2006-01-02",
		"2006-1-2",
	}
	var t time.Time
	var err error
	formatted = strings.TrimSpace(formatted)
	for _, layout := range layouts {
		t, err = time.Parse(layout, formatted)
		if !t.IsZero() {
			break
		}
	}
	return t, err
}

func parseStrToInt(digit string) int64 {
	i64, err := strconv.ParseInt(digit, 10, 64)
	if err != nil {
		return 0
	}
	return i64
}
