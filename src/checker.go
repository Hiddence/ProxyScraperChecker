package src

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// ProxyLocation contains geolocation information
type ProxyLocation struct {
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	City        string `json:"city"`
	Region      string `json:"regionName"`
}

// CheckResult represents the result of a proxy check
type CheckResult struct {
	Proxy     string
	Working   bool
	Type      ProxyType
	ProxyIP   string
	Speed     time.Duration
	Anonymous bool
	Location  *ProxyLocation
}

// ProxyInfo contains detailed information about a proxy
type ProxyInfo struct {
	IP        string
	Port      string
	Type      ProxyType
	Anonymous bool
	Speed     time.Duration
}

// ProxyChecker handles the checking of proxies
type ProxyChecker struct {
	config      *Config
	httpClient  *http.Client
	ResultChan  chan CheckResult
	progressMu  sync.Mutex
	checkedHTTP int
	checkedSOCKS5 int
	workingHTTP int
	workingSOCKS5 int
	totalHTTP int
	totalSOCKS5 int
}

// NewProxyChecker creates a new ProxyChecker instance
func NewProxyChecker(config *Config) *ProxyChecker {
	return &ProxyChecker{
		config:     config,
		ResultChan: make(chan CheckResult, 100),
	}
}

// formatProxyOutput formats proxy information for output
func (c *ProxyChecker) formatProxyOutput(result CheckResult) string {
	if !c.config.Checker.StrictCheck || !c.config.Checker.DetailedOutput {
		return result.Proxy
	}

	// Format: proxy|ip|country|city|speed|anonymous
	speed := result.Speed.Round(time.Millisecond).String()
	anonymous := "No"
	if result.Anonymous {
		anonymous = "Yes"
	}

	location := "Unknown"
	if result.Location != nil {
		if result.Location.City != "" {
			location = fmt.Sprintf("%s, %s", result.Location.City, result.Location.Country)
		} else {
			location = result.Location.Country
		}
	}

	return fmt.Sprintf("%s|%s|%s|%s|%s", 
		result.Proxy,
		result.ProxyIP,
		location,
		speed,
		anonymous,
	)
}

// CheckProxies checks a list of proxies concurrently
func (c *ProxyChecker) CheckProxies(httpProxies, socks5Proxies []string) {
	c.totalHTTP = len(httpProxies)
	c.totalSOCKS5 = len(socks5Proxies)
	
	var wg sync.WaitGroup
	semHTTP := make(chan struct{}, c.config.Checker.ConcurrentHTTP)
	semSOCKS5 := make(chan struct{}, c.config.Checker.ConcurrentSOCKS5)

	// Create mutex for file writing
	var httpMu sync.Mutex
	var socks5Mu sync.Mutex

	// Write headers if detailed output is enabled
	if c.config.Checker.StrictCheck && c.config.Checker.DetailedOutput {
		header := "Proxy|IP|Location|Response Time|Anonymous"
		if err := WriteFile(filepath.Join("out", "http.txt"), header); err != nil {
			log.Printf("Error writing HTTP header: %v", err)
		}
		if err := WriteFile(filepath.Join("out", "socks5.txt"), header); err != nil {
			log.Printf("Error writing SOCKS5 header: %v", err)
		}
	}

	// Start HTTP proxy checks
	for _, proxy := range httpProxies {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			semHTTP <- struct{}{}
			defer func() { <-semHTTP }()
			if result := c.checkHTTPProxy(p); result.Working {
				output := c.formatProxyOutput(result)
				if err := AppendLine(filepath.Join("out", "http.txt"), output, &httpMu); err != nil {
					log.Printf("Error saving HTTP proxy: %v", err)
				}
			}
		}(proxy)
	}

	// Start SOCKS5 proxy checks
	for _, proxy := range socks5Proxies {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			semSOCKS5 <- struct{}{}
			defer func() { <-semSOCKS5 }()
			if result := c.checkSOCKS5Proxy(p); result.Working {
				output := c.formatProxyOutput(result)
				if err := AppendLine(filepath.Join("out", "socks5.txt"), output, &socks5Mu); err != nil {
					log.Printf("Error saving SOCKS5 proxy: %v", err)
				}
			}
		}(proxy)
	}

	// Start progress display
	go c.displayProgress()

	wg.Wait()
	close(c.ResultChan)
}

// testProxy tests if a proxy is working
func (c *ProxyChecker) testProxy(client *http.Client) (bool, string, time.Duration, bool, *ProxyLocation) {
	if !c.config.Checker.StrictCheck {
		// Simple check - just verify if proxy returns 200 OK
		req, err := http.NewRequest("GET", c.config.Checker.TestURL, nil)
		if err != nil {
			return false, "", 0, false, nil
		}

		req.Header.Set("User-Agent", c.config.Checker.UserAgent)
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return false, "", 0, false, nil
		}
		defer resp.Body.Close()

		return resp.StatusCode == http.StatusOK, "", time.Since(start), false, nil
	}

	// Strict check with multiple criteria
	var proxyIP string
	var location *ProxyLocation
	var isAnonymous bool
	var totalTime time.Duration

	// 1. Check IP and location using ip-api.com
	req, err := http.NewRequest("GET", "http://ip-api.com/json", nil)
	if err != nil {
		return false, "", 0, false, nil
	}

	req.Header.Set("User-Agent", c.config.Checker.UserAgent)
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return false, "", 0, false, nil
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return false, "", 0, false, nil
	}

	// Parse IP-API response
	var ipData struct {
		Status      string  `json:"status"`
		Country     string  `json:"country"`
		CountryCode string  `json:"countryCode"`
		Region      string  `json:"regionName"`
		City        string  `json:"city"`
		Query       string  `json:"query"`
	}

	if err := json.Unmarshal(body, &ipData); err != nil || ipData.Status != "success" {
		return false, "", 0, false, nil
	}

	proxyIP = ipData.Query
	location = &ProxyLocation{
		Country:     ipData.Country,
		CountryCode: ipData.CountryCode,
		City:        ipData.City,
		Region:      ipData.Region,
	}

	// 2. Check anonymity by comparing headers
	req, err = http.NewRequest("GET", "https://httpbin.org/headers", nil)
	if err != nil {
		return false, "", 0, false, nil
	}

	req.Header.Set("User-Agent", c.config.Checker.UserAgent)
	resp, err = client.Do(req)
	if err != nil {
		return false, "", 0, false, nil
	}

	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return false, "", 0, false, nil
	}

	var headers struct {
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(body, &headers); err != nil {
		return false, "", 0, false, nil
	}

	// Check if proxy reveals real IP in headers
	isAnonymous = true
	for _, v := range headers.Headers {
		if strings.Contains(v, proxyIP) {
			isAnonymous = false
			break
		}
	}

	totalTime = time.Since(start)
	// Proxy is considered working if:
	// 1. Response time is under 2 seconds
	// 2. We got valid IP and location data
	working := totalTime < 2*time.Second && proxyIP != ""

	return working, proxyIP, totalTime, isAnonymous, location
}

// checkHTTPProxy checks a single HTTP proxy
func (c *ProxyChecker) checkHTTPProxy(proxyStr string) CheckResult {
	proxyURL, err := url.Parse("http://" + proxyStr)
	if err != nil {
		log.Printf("Error parsing HTTP proxy %s: %v", proxyStr, err)
		c.updateProgress(ProxyTypeHTTP, false)
		return CheckResult{Proxy: proxyStr, Working: false, Type: ProxyTypeHTTP}
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout:   c.config.Checker.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   c.config.Checker.ConnectTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: c.config.Checker.Timeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   c.config.Checker.Timeout,
	}

	working, proxyIP, speed, anonymous, location := c.testProxy(client)
	result := CheckResult{
		Proxy:     proxyStr,
		Working:   working,
		Type:      ProxyTypeHTTP,
		ProxyIP:   proxyIP,
		Speed:     speed,
		Anonymous: anonymous,
		Location:  location,
	}
	c.ResultChan <- result
	c.updateProgress(ProxyTypeHTTP, working)
	return result
}

// checkSOCKS5Proxy checks a single SOCKS5 proxy
func (c *ProxyChecker) checkSOCKS5Proxy(proxyStr string) CheckResult {
	dialer, err := proxy.SOCKS5("tcp", proxyStr, nil, &net.Dialer{
		Timeout:   c.config.Checker.ConnectTimeout,
		KeepAlive: 30 * time.Second,
	})
	if err != nil {
		log.Printf("Error creating SOCKS5 dialer for %s: %v", proxyStr, err)
		c.updateProgress(ProxyTypeSOCKS5, false)
		return CheckResult{Proxy: proxyStr, Working: false, Type: ProxyTypeSOCKS5}
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   c.config.Checker.ConnectTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: c.config.Checker.Timeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   c.config.Checker.Timeout,
	}

	working, proxyIP, speed, anonymous, location := c.testProxy(client)
	result := CheckResult{
		Proxy:     proxyStr,
		Working:   working,
		Type:      ProxyTypeSOCKS5,
		ProxyIP:   proxyIP,
		Speed:     speed,
		Anonymous: anonymous,
		Location:  location,
	}
	c.ResultChan <- result
	c.updateProgress(ProxyTypeSOCKS5, working)
	return result
}

// updateProgress updates the progress counters
func (c *ProxyChecker) updateProgress(proxyType ProxyType, working bool) {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()

	switch proxyType {
	case ProxyTypeHTTP:
		c.checkedHTTP++
		if working {
			c.workingHTTP++
		}
	case ProxyTypeSOCKS5:
		c.checkedSOCKS5++
		if working {
			c.workingSOCKS5++
		}
	}
}

// displayProgress displays the progress of proxy checking
func (c *ProxyChecker) displayProgress() {
	for {
		c.progressMu.Lock()
		if c.checkedHTTP == c.totalHTTP && c.checkedSOCKS5 == c.totalSOCKS5 {
			c.progressMu.Unlock()
			break
		}

		httpPercentage := float64(c.checkedHTTP) / float64(c.totalHTTP) * 100
		socks5Percentage := float64(c.checkedSOCKS5) / float64(c.totalSOCKS5) * 100

		fmt.Print("\033[1A\033[K")
		fmt.Printf("\rHTTP [%d/%d] - Working: %d %s %.0f%%", 
			c.checkedHTTP, c.totalHTTP, c.workingHTTP, 
			ProgressBar(httpPercentage, 30), httpPercentage)
		
		fmt.Print("\n")
		fmt.Printf("\rSOCKS5 [%d/%d] - Working: %d %s %.0f%%", 
			c.checkedSOCKS5, c.totalSOCKS5, c.workingSOCKS5, 
			ProgressBar(socks5Percentage, 30), socks5Percentage)

		c.progressMu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	// Final progress update
	fmt.Printf("\n✓ Found %d working HTTP proxies\n", c.workingHTTP)
	fmt.Printf("✓ Found %d working SOCKS5 proxies\n", c.workingSOCKS5)
} 