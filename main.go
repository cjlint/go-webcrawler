package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"net/url"

	"golang.org/x/net/html"
)

var logMutex sync.Mutex

type urlInfo struct {
	val   string
	depth int
}

func standardizeURL(urlObj *url.URL) string {
	return fmt.Sprintf("https://%s%s", urlObj.Hostname(), urlObj.Path)
}

func crawl(targetURL string, depth int, ch chan urlInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	resp, err := http.Get(targetURL)
	if err != nil {
		log.Println("Error while fetching URL", targetURL, err)
		return
	}
	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Println("Failed to parse body from URL", targetURL, err)
		return
	}

	// Many URLs will be the same after sanitizing, keep a local map
	// of seen URLs to reduce duplicates in logs
	seenURLs := map[string]bool{}
	var urls []string

	// HTML parsing code adapted from
	// https://pkg.go.dev/golang.org/x/net/html#example-Parse
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			// found anchor element
			for _, a := range n.Attr {
				if a.Key == "href" {
					// Parse url so we can standardize it
					urlObj, err := url.Parse(a.Val)
					if err != nil {
						log.Println("Error parsing url", a.Val, err)
					} else if urlObj.Scheme == "https" {
						// Skip anything that isn't an absolute https url
						// Standardize URL to prevent crawling the same URL multiple times
						// for example, ignore query parameters and standardize path +
						// trailing slash
						standardizedURL := standardizeURL(urlObj)
						if !seenURLs[standardizedURL] {
							urls = append(urls, standardizedURL)
							ch <- urlInfo{standardizedURL, depth + 1}
						}
						seenURLs[standardizedURL] = true
					}
					break
				}
			}
		}
		// Read html recursively. Iteratively would be better in case of
		// large html files that use excess memory, but this solution
		// works for now :)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// Use lock to make sure that different goroutines don't mix logs,
	// which could cause confusing and incorrect log output
	logMutex.Lock()
	defer logMutex.Unlock()
	log.Printf("%s (depth %d)\n", targetURL, depth)
	for _, childURL := range urls {
		log.Printf("    %s\n", childURL)
	}
}

func aggregateURLs(baseURL string, maxDepth int) {
	log.Println("Max depth set to", maxDepth)

	// Standardize baseURL, assume https scheme
	urlObj, err := url.Parse(baseURL)
	if err != nil {
		log.Fatal("Error parsing base URL", baseURL, err)
	}
	standardizedURL := standardizeURL(urlObj)

	crawled := map[string]bool{standardizedURL: true}
	// Buffered channel to minimize blocking
	urlAggregation := make(chan urlInfo, 100)
	var wg sync.WaitGroup

	wg.Add(1)
	go crawl(standardizedURL, 0, urlAggregation, &wg)

	// Use wg to detect when there are no more running crawl operations,
	// then close the url aggregation channel to stop the process
	//
	// This urlAggregation channel method is iterative instead of
	// recursive, allowing the program to run longer without
	// worrying about memory issues
	go func() {
		wg.Wait()
		close(urlAggregation)
	}()

	for url := range urlAggregation {
		if !crawled[url.val] && url.depth < maxDepth {
			wg.Add(1)
			crawled[url.val] = true
			go crawl(url.val, url.depth, urlAggregation, &wg)
		}
	}
}

func main() {
	url := flag.String("url", "", "REQUIRED URL to begin parsing")
	depth := flag.Int("depth", 2, "Max depth for crawling. Set to 0 for no max depth")
	flag.Parse()
	if *url == "" {
		flag.Usage()
		os.Exit(1)
	}
	aggregateURLs(*url, *depth)
}
