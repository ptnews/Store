package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

func main() {
	data, _ := os.ReadFile("feeds.txt")
	feeds := strings.Split(string(data), "\n")

	contentDir := "content/news"
	os.RemoveAll(contentDir)
	os.MkdirAll(contentDir, os.ModePerm)



	var allPosts []map[string]any
	reImg := regexp.MustCompile(`src=["']([^"']+\.(jpg|jpeg|png|gif|webp))["']`)

	for _, url := range feeds {
		url = strings.TrimSpace(url)
		if url == "" || strings.HasPrefix(url, "#") {
			continue
		}
		fmt.Println("Fetching:", url)

		resp, err := http.Get("https://corsproxy.io/?" + url)
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
			source.WriteString(strings.ReplaceAll(url, "www.", "")) // Corrected line to use URL from `feeds` slice

			cleanedTitle := strings.ReplaceAll(strings.ReplaceAll(item.Title, `"`, `\"`), "\n", " ")
			cleanedDescText := strings.ReplaceAll(strings.ReplaceAll(descText, `"`, `\"`), "\n", " ")

			frontmatter := fmt.Sprintf(`---
title: "%s"
date: %s
description: "%s"
image: "%s"
link: "%s"
source: "%s"
draft: false
---
%s
`, cleanedTitle,
				pubDate,
				cleanedDescText,
				image,
				item.Link,
				source.String(),
				item.Content)

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
