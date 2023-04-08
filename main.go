package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"

	"net/url"

	"golang.org/x/net/html"
)

type urlResults struct {
	baseURL   string
	childURLs []string
	depth     int
}

type urlInfo struct {
	val   string
	depth int
}

func standardizeURL(urlObj *url.URL) string {
	// Remove trailing / from end of path, otherwise foo.com and foo.com/
	// will be unnecessarily treated as different URLs
	trimmedPath := strings.TrimRight(urlObj.Path, "/")
	return fmt.Sprintf("https://%s%s", urlObj.Hostname(), trimmedPath)
}

func parseURLs(doc *html.Node) []string {
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
					urlObj, err := url.Parse(strings.TrimSpace(a.Val))
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

	return urls
}

func crawlWorker(client *http.Client, urlsToCrawl chan urlInfo, results chan urlResults) {
	for url := range urlsToCrawl {
		resp, err := client.Get(url.val)
		if err != nil {
			log.Println("Error while fetching URL", url, err)
			return
		}
		doc, err := html.Parse(resp.Body)
		if err != nil {
			log.Println("Failed to parse body from URL", url, err)
			return
		}

		childURLs := parseURLs(doc)

		results <- urlResults{url.val, childURLs, url.depth}
	}
}

func processResults(wg *sync.WaitGroup, urlsToCrawl chan urlInfo, results chan urlResults, maxDepth int) {
	// Background function that prints logs in synchronous order,
	// then sends child urls to next channel to be processed
	//
	// The waitgroup watches urlsToCrawl -- once it is empty the program can end.
	crawled := map[string]bool{}
	for info := range results {
		crawled[info.baseURL] = true
		log.Printf("%s (depth %d)\n", info.baseURL, info.depth)
		for _, childURL := range info.childURLs {
			log.Printf("    %s\n", childURL)
		}
		for _, childURL := range info.childURLs {
			if !crawled[childURL] && (info.depth < maxDepth || maxDepth == 0) {
				// select statement ensures that this operation never blocks,
				// even if it means we have to start throwing away URLs
				// that don't fit in the buffer
				select {
				case urlsToCrawl <- urlInfo{childURL, info.depth + 1}:
					wg.Add(1)
				default:
					log.Println("URL buffer is full, discarding URL", childURL)
				}
			}
		}
		wg.Done()
	}
	close(urlsToCrawl)
}

func recommendedWorkers(maxDepth int) int {
	if maxDepth == 0 {
		return 1000
	}
	return int(math.Min(math.Pow10(maxDepth-1), 1000))
}

func crawl(baseURL string, maxDepth, maxWorkers int) {
	log.Println("Max depth set to", maxDepth)
	if maxDepth == 0 {
		log.Println("No max depth specified -- program may not terminate")
	}
	if maxWorkers == 0 {
		maxWorkers = recommendedWorkers(maxDepth)
	}
	log.Println("Number of workers set to", maxWorkers)
	// Standardize baseURL, assume https scheme
	urlObj, err := url.Parse(baseURL)
	if err != nil {
		log.Fatal("Error parsing base URL", baseURL, err)
	}
	standardizedURL := standardizeURL(urlObj)

	// buffered channel prevents deadlocking, because the results
	// process and crawling process feed into each other
	urlsToCrawl := make(chan urlInfo, 1000*maxWorkers)
	results := make(chan urlResults)
	var wg sync.WaitGroup
	// Make sure we wait for the base URL crawl to finish
	wg.Add(1)
	// First initialize channel with base URL
	go func() {
		urlsToCrawl <- urlInfo{standardizedURL, 1}
	}()

	go processResults(&wg, urlsToCrawl, results, maxDepth)

	// Create custom http client that disables keepalives
	// to conserve resources
	tr := &http.Transport{
		DisableKeepAlives: true,
	}
	client := &http.Client{Transport: tr}

	// Spawn the appropriate number of crawl workers
	for i := 0; i < maxWorkers; i++ {
		go crawlWorker(client, urlsToCrawl, results)
	}

	// In the main thread, use wg to detect when there are no more
	// urls to crawl, then close the channel to stop the workers
	wg.Wait()
	log.Println("No more URLs to crawl, ending program")
	close(results)

}

func main() {
	url := flag.String("url", "", "REQUIRED URL to begin parsing")
	depth := flag.Int("depth", 3, "Max depth for crawling. Set to 0 for no max depth")
	workers := flag.Int("workers", 0, "Max number of workers in the pool for crawling. A reasonable default will be chosen based on depth setting")
	flag.Parse()
	if *url == "" {
		flag.Usage()
		os.Exit(1)
	}
	crawl(*url, *depth, *workers)
}
