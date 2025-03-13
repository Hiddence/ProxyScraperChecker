package src

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ReadLines reads a file and returns lines as a string slice
func ReadLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// WriteLines writes a slice of strings to a file, one string per line
func WriteLines(path string, lines []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, line := range lines {
		_, err := file.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

// AppendLine appends a single line to a file with thread safety
func AppendLine(path string, line string, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(line + "\n")
	return err
}

// RemoveDuplicates removes duplicate proxies and returns unique ones
func RemoveDuplicates(proxies []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, proxy := range proxies {
		if _, exists := seen[proxy]; !exists {
			seen[proxy] = struct{}{}
			result = append(result, proxy)
		}
	}

	return result
}

// ClearLine clears the current line in the console
func ClearLine() {
	fmt.Print("\r\033[K")
}

// ProgressBar generates a progress bar string
func ProgressBar(percentage float64, width int) string {
	filled := int(percentage / 100 * float64(width))
	if filled > width {
		filled = width
	}
	
	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"
	
	return bar
}

// WriteFile writes content to a file
func WriteFile(path string, content string) error {
	return os.WriteFile(path, []byte(content+"\n"), 0644)
} 