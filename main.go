package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"ProxyScraperChecker/src"
)

func main() {
	// Parse command line flags
	strictCheck := flag.Bool("strict", false, "Enable strict proxy checking")
	detailedOutput := flag.Bool("detailed", false, "Show detailed checking results")
	flag.Parse()

	// Set up logging to file
	logFile, err := os.OpenFile("proxy_checker.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	// Load configuration
	config, err := src.LoadConfig("config.yaml")
	if err != nil {
		log.Printf("Error loading config: %v", err)
		return
	}

	// Update checker configuration with command line flags
	config.Checker.StrictCheck = *strictCheck
	config.Checker.DetailedOutput = *detailedOutput

	// Create output directory if it doesn't exist
	if err := os.MkdirAll("out", 0755); err != nil {
		log.Printf("Error creating output directory: %v", err)
		return
	}

	fmt.Println("🚀 Proxy Scraper and Checker Started")
	
	// Display active parameters
	if *strictCheck || *detailedOutput {
		fmt.Println("Active parameters:")
		if *strictCheck {
			fmt.Println("  • Strict checking mode enabled")
		}
		if *detailedOutput {
			fmt.Println("  • Detailed output mode enabled")
		}
		fmt.Println()
	}

	// Scrape HTTP proxies
	httpSources, err := src.ReadLines(filepath.Join("sources", "http.txt"))
	if err != nil {
		log.Printf("Error reading HTTP sources: %v", err)
		return
	}

	httpProxies := src.ScrapeProxies(httpSources, config.Scraper.UserAgents, config.Scraper.Timeout, "HTTP", config.Scraper.Concurrent)

	// Scrape SOCKS5 proxies
	socks5Sources, err := src.ReadLines(filepath.Join("sources", "socks5.txt"))
	if err != nil {
		log.Printf("Error reading SOCKS5 sources: %v", err)
		return
	}

	socks5Proxies := src.ScrapeProxies(socks5Sources, config.Scraper.UserAgents, config.Scraper.Timeout, "SOCKS5", config.Scraper.Concurrent)

	// Read existing proxies
	existingHTTP, _ := src.ReadLines(filepath.Join("out", "http.txt"))
	existingSOCKS5, _ := src.ReadLines(filepath.Join("out", "socks5.txt"))

	// Add existing proxies
	if len(existingHTTP) > 0 {
		fmt.Printf("ℹ️ Found %d existing HTTP proxies\n", len(existingHTTP))
		httpProxies = append(httpProxies, existingHTTP...)
	}
	if len(existingSOCKS5) > 0 {
		fmt.Printf("ℹ️ Found %d existing SOCKS5 proxies\n", len(existingSOCKS5))
		socks5Proxies = append(socks5Proxies, existingSOCKS5...)
	}

	// Remove duplicates
	httpProxies = src.RemoveDuplicates(httpProxies)
	socks5Proxies = src.RemoveDuplicates(socks5Proxies)

	// Clear existing output files
	if err := os.WriteFile(filepath.Join("out", "http.txt"), []byte{}, 0644); err != nil {
		log.Printf("Error clearing HTTP output file: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join("out", "socks5.txt"), []byte{}, 0644); err != nil {
		log.Printf("Error clearing SOCKS5 output file: %v", err)
		return
	}

	fmt.Printf("✅ Total %d HTTP proxies to check\n", len(httpProxies))
	fmt.Printf("✅ Total %d SOCKS5 proxies to check\n", len(socks5Proxies))
	fmt.Println("🔍 Checking proxies...")

	// Create checker and start checking
	checker := src.NewProxyChecker(config)
	go func() {
		for result := range checker.ResultChan {
			// Just receive results to prevent blocking
			_ = result
		}
	}()

	checker.CheckProxies(httpProxies, socks5Proxies)
	fmt.Println("\n✨ Proxy scraping and checking completed")
}