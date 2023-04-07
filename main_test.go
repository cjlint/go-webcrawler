package main

import (
	"net/url"
	"strings"
	"testing"

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
