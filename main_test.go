package main

import (
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestStandardizeURL(t *testing.T) {
	tests := []struct {
		url, expected string
	}{
		{"foo.com", "https://foo.com"},
		{"foo", "https://foo"},
		{"foo.io", "https://foo.io"},
		{"http://foo.com", "https://foo.com"},
		{"https://foo.com", "https://foo.com"},
		{"foo.com/", "https://foo.com"},
		{"foo.com/////", "https://foo.com"},
		{"foo.com/a/b/c/", "https://foo.com/a/b/c"},
		{"foo.com?a=b&c=d", "https://foo.com"},
		{"foo.com/?a=b&c=d", "https://foo.com"},
		{"foo.com#tag", "https://foo.com"},
		{"foo.com/#tag?a=b&c=d", "https://foo.com"},
	}
	for _, tt := range tests {
		t.Run("Standardize URL "+tt.url, func(t *testing.T) {
			u, _ := url.Parse(tt.url)
			if result := standardizeURL(u); result != tt.expected {
				t.Errorf("standardizeURL() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestParseURLs(t *testing.T) {
	tests := []struct {
		htmlBody string
		expected []string
	}{
		{`<a href="https://foo.com">`, []string{"https://foo.com"}},
		{`<a target="_" x-other-attr="https://bar.com" href="https://foo.com">`,
			[]string{"https://foo.com"}},
		{`<a href="  https://foo.com  ">`, []string{"https://foo.com"}},
		{`<a href="foo.com">`, []string{}},
		{`<a href="not a url">`, []string{}},
		{`<a href="http://foo.com">`, []string{}},
		{`<a href="mailto:me@foo.com">`, []string{}},
		{`<a href="/relativepath">`, []string{}},
		{`<nav>
			<a href="https://foo.com">
			<a href="https://foo.com">
		</nav>`, []string{"https://foo.com"}},
		{`<nav>
			<a href="https://foo.com">
			<a href="https://bar.com">
		</nav>
		<a href="https://baz.com">
		<a href="not a url">`, []string{
			"https://foo.com", "https://bar.com", "https://baz.com",
		}},
	}
	for _, tt := range tests {
		t.Run("Test "+tt.htmlBody, func(t *testing.T) {
			doc, _ := html.Parse(strings.NewReader(tt.htmlBody))
			result := parseURLs(doc)
			equal := len(result) == len(tt.expected)
			for i, expected := range tt.expected {
				equal = equal && result[i] == expected
			}
			if !equal {
				t.Errorf("parseURLs() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestProcessResults(t *testing.T) {
	tests := []struct {
		name           string
		resultsInput   []urlResults
		expectedOutput []urlInfo
	}{
		{"test basic", []urlResults{
			{"https://foo.com", []string{"https://a.com", "https://b.com"}, 1},
			{"https://a.com", []string{"https://c.com", "https://d.com"}, 2},
		}, []urlInfo{
			{"https://a.com", 2},
			{"https://b.com", 2},
			{"https://c.com", 3},
			{"https://d.com", 3},
		}},
		{"test ignores already crawled", []urlResults{
			{"https://foo.com", []string{"https://a.com", "https://foo.com"}, 1},
			{"https://a.com", []string{"https://a.com", "https://b.com"}, 2},
		}, []urlInfo{
			{"https://a.com", 2},
			{"https://b.com", 3},
		}},
		{"test overflow is ignored", []urlResults{
			{"https://foo.com", []string{
				"https://a.com",
				"https://b.com",
				"https://c.com",
				"https://d.com",
				"https://e.com",
				"https://f.com",
				"https://g.com",
			}, 1},
		}, []urlInfo{
			{"https://a.com", 2},
			{"https://b.com", 2},
			{"https://c.com", 2},
			{"https://d.com", 2},
			{"https://e.com", 2},
		}},
		{"test max depth is ignored", []urlResults{
			{"https://foo.com", []string{"https://a.com", "https://b.com"}, 3},
		}, []urlInfo{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			urlsToCrawl := make(chan urlInfo, 5)
			results := make(chan urlResults)
			wg.Add(1)
			go processResults(&wg, urlsToCrawl, results, 3)

			for _, result := range tt.resultsInput {
				results <- result
			}
			close(results)

			time.Sleep(100 * time.Millisecond)
			for i, expectedURL := range tt.expectedOutput {
				url, ok := <-urlsToCrawl
				if url != expectedURL {
					t.Errorf("urlsToCrawl item %d = %v, expected %v", i, url, expectedURL)
				}
				if !ok {
					t.Errorf("Not enough items in urlsToCrawl")
				}
			}
			if len(urlsToCrawl) != 0 {
				remainingURLs := []urlInfo{}
				for i := 0; i < len(urlsToCrawl); i++ {
					remainingURLs = append(remainingURLs, <-urlsToCrawl)
				}
				t.Errorf("Too many urls left in urlsToCrawl %v", remainingURLs)
			}
		})
	}
}
