package src

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ProxyData represents JSON proxy data structure with flexible fields
type ProxyData struct {
	Data []struct {
		IP   string `json:"ip"`
		Port string `json:"port,omitempty"`
		// Additional fields that might contain port information
		ProxyPort string `json:"proxy_port,omitempty"`
		PortNum   string `json:"port_num,omitempty"`
		PortNumber string `json:"port_number,omitempty"`
	} `json:"data"`
}

// truncateURL shortens a URL if it exceeds maxLength
func truncateURL(url string, maxLength int) string {
	if len(url) <= maxLength {
		return url
	}
	return url[:maxLength-3] + "..."
}

// normalizeProxy converts various proxy formats to IP:PORT format
func normalizeProxy(proxy string) string {
	// Remove protocol prefix if exists
	proxy = strings.TrimPrefix(proxy, "http://")
	proxy = strings.TrimPrefix(proxy, "https://")
	proxy = strings.TrimPrefix(proxy, "socks4://")
	proxy = strings.TrimPrefix(proxy, "socks5://")

	// Try to parse as JSON
	if strings.HasPrefix(proxy, "{") {
		var data ProxyData
		if err := json.Unmarshal([]byte(proxy), &data); err == nil && len(data.Data) > 0 {
			ip := data.Data[0].IP
			// Try different port field names
			port := data.Data[0].Port
			if port == "" {
				port = data.Data[0].ProxyPort
			}
			if port == "" {
				port = data.Data[0].PortNum
			}
			if port == "" {
				port = data.Data[0].PortNumber
			}
			if port == "" {
				port = "80" // Default port if not specified
			}
			return fmt.Sprintf("%s:%s", ip, port)
		}
	}

	// Try to find IP:PORT pattern
	re := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d+)`)
	if matches := re.FindStringSubmatch(proxy); matches != nil {
		return fmt.Sprintf("%s:%s", matches[1], matches[2])
	}

	// Try to find IP and PORT separately
	ipRe := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	portRe := regexp.MustCompile(`\d{1,5}`)
	
	ip := ipRe.FindString(proxy)
	port := portRe.FindString(proxy)
	
	if ip != "" && port != "" {
		return fmt.Sprintf("%s:%s", ip, port)
	}

	return ""
}

// isValidProxy checks if a proxy string is valid and returns normalized format
func isValidProxy(proxy string) (string, bool) {
	if proxy == "" {
		return "", false
	}

	// Try to normalize the proxy string
	normalized := normalizeProxy(proxy)
	if normalized == "" {
		return "", false
	}

	// Validate the normalized format
	parts := strings.Split(normalized, ":")
	if len(parts) != 2 {
		return "", false
	}

	// Check if port is numeric and in valid range
	port := parts[1]
	if len(port) == 0 || len(port) > 5 {
		return "", false
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return "", false
		}
	}

	return normalized, true
}

// ScrapeProxies scrapes proxies from a list of URLs
func ScrapeProxies(urls []string, userAgents []string, timeout time.Duration, proxyType string, concurrent int) []string {
	var proxies []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrent)
	var completedURLs int
	var totalFound int

	client := &http.Client{
		Timeout: timeout,
	}

	// Print initial message
	fmt.Printf("Starting %s proxy scraping...\n", proxyType)
	
	// Start progress display goroutine
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				if completedURLs == len(urls) {
					mu.Unlock()
					return
				}
				fmt.Print("\n\033[1A\033[K") // Move cursor up and clear line
				fmt.Printf("\r✓ Scraped %d %s proxies [%d/%d]", 
					totalFound, proxyType, completedURLs, len(urls))
				mu.Unlock()
			}
		}
	}()
	
	for i, url := range urls {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				log.Printf("Error creating request for %s: %v", url, err)
				return
			}

			// Rotate user agents
			userAgent := userAgents[i%len(userAgents)]
			req.Header.Set("User-Agent", userAgent)

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Error fetching %s: %v", url, err)
				return
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("Error reading response from %s: %v", url, err)
				return
			}

			// Split response by newlines and filter valid proxies
			var localProxies []string
			lines := strings.Split(string(body), "\n")
			for _, line := range lines {
				proxy := strings.TrimSpace(line)
				if normalized, ok := isValidProxy(proxy); ok {
					localProxies = append(localProxies, normalized)
				}
			}

			// Update proxies slice thread-safely
			mu.Lock()
			proxies = append(proxies, localProxies...)
			completedURLs++
			totalFound = len(proxies)
			mu.Unlock()
		}(i, url)
	}

	wg.Wait()
	close(done)
	fmt.Print("\n\033[1A\033[K")
	fmt.Printf("✓ Scraped %d %s proxies [%d/%d]\n", len(proxies), proxyType, completedURLs, len(urls))
	return proxies
} 