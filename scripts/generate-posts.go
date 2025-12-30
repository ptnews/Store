package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			PubDate     string `xml:"pubDate"`
			Description string `xml:"description"`
			Content     string `xml:"encoded"`
			Enclosure   struct {
				URL  string `xml:"url,attr"`
				Type string `xml:"type,attr"`
			} `xml:"enclosure"`
			Media struct {
				URL string `xml:"url,attr"`
			} `xml:"media:content"`
		} `xml:"item"`
	} `xml:"channel"`
}

func extractTags(title string) []string {
	// a simple approach to extract tags from title
	// it will not be perfect, but it's better than nothing
	re := regexp.MustCompile(`[^\w]`)
	words := re.Split(strings.ToLower(title), -1)

	// remove stop words
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true, "in": true, "on": true, "for": true, "to": true, "is": true, "of": true,
		"with": true, "as": true, "by": true, "at": true, "from": true, "says": true, "after": true, "it": true, "its": true,
	}
	var tags []string
	for _, word := range words {
		if len(word) > 4 && !stopWords[word] {
			tags = append(tags, word)
		}
	}

	// sort by length desc
	sort.Slice(tags, func(i, j int) bool {
		return len(tags[i]) > len(tags[j])
	})

	// return top 5
	if len(tags) > 5 {
		return tags[:5]
	}
	return tags
}

func formatTags(tags []string) string {
	var builder strings.Builder
	for _, tag := range tags {
		builder.WriteString(fmt.Sprintf("- '%s'\n", tag))
	}
	return builder.String()
}

func main() {
	data, _ := os.ReadFile("feeds.txt")
	feeds := strings.Split(string(data), "\n")

	contentDir := "content/news"
	os.RemoveAll(contentDir)
	os.MkdirAll(contentDir, os.ModePerm)

	var allPosts []map[string]any
	reImg := regexp.MustCompile(`src=["']([^"']+\.(jpg|jpeg|png|gif|webp))["']`)

	for _, feedURL := range feeds {
		feedURL = strings.TrimSpace(feedURL)
		if feedURL == "" || strings.HasPrefix(feedURL, "#") {
			continue
		}
		fmt.Println("Fetching:", feedURL)

		resp, err := http.Get("https://corsproxy.io/?" + feedURL)
		if err != nil || resp.StatusCode != 200 {
			fmt.Println("Error:", err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var rss RSS
		xml.Unmarshal(body, &rss)

		for _, item := range rss.Channel.Items {
			slug := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(item.Title), "-")
			slug = regexp.MustCompile(`^-+|-+$`).ReplaceAllString(slug, "")
			if len(slug) > 200 {
				slug = slug[:200]
			}
			if slug == "" {
				slug = time.Now().Format("20060102150405")
			}

			pubDate := time.Now().Format("2006-01-02")
			if item.PubDate != "" {
				if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
					pubDate = t.Format("2006-01-02")
				}
			}

			image := ""
			if item.Enclosure.URL != "" && strings.Contains(item.Enclosure.Type, "image") {
				image = item.Enclosure.URL
			} else if item.Media.URL != "" {
				image = item.Media.URL
			} else {
				desc := item.Content
				if desc == "" {
					desc = item.Description
				}
				if match := reImg.FindStringSubmatch(desc); len(match) > 1 {
					image = match[1]
				}
			}

			descText := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(item.Description, "")
			if len(descText) > 300 {
				descText = descText[:300] + "..."
			}

			source := new(strings.Builder)
			source.WriteString(strings.ReplaceAll(feedURL, "www.", "")) // Corrected line to use URL from `feeds` slice

			cleanedTitle := strings.ReplaceAll(item.Title, "\"", "\\\"")
			cleanedDescText := strings.ReplaceAll(descText, "\"", "\\\"")

			// Add categories and tags
			parsedURL, err := url.Parse(feedURL)
			var category string
			if err == nil {
				category = strings.ReplaceAll(parsedURL.Hostname(), "www.", "")
			}

			tags := extractTags(item.Title)
			
			frontmatter := fmt.Sprintf(`---
title: "%s"
date: %s
description: "%s"
image: '%s'
link: '%s'
source: '%s'
categories:
- '%s'
tags:
%s
draft: false
---
%s
`, cleanedTitle, pubDate, cleanedDescText, image, item.Link, source.String(), category, formatTags(tags), item.Content)

			os.WriteFile(filepath.Join(contentDir, slug+".md"), []byte(frontmatter), 0644)

			allPosts = append(allPosts, map[string]any{
				"slug":        slug,
				"title":       item.Title,
				"description": descText,
				"pubDate":     pubDate,
				"image":       image,
				"link":        item.Link,
				"source":      source.String(),
			})
		}
	}

	// Generate posts.json for client-side
	jsonData, err := json.MarshalIndent(allPosts, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling json:", err)
	}
	os.WriteFile("static/data/posts.json", jsonData, 0644)

	fmt.Printf("Generated %d articles\n", len(allPosts))
}