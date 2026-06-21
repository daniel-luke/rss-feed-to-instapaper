package feed

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

type Item struct {
	GUID        string
	URL         string
	Title       string
	PublishedAt *time.Time
}

func FetchItems(feedURL string) ([]Item, error) {
	fp := gofeed.NewParser()
	fp.Client = &http.Client{Timeout: 30 * time.Second}
	parsed, err := fp.ParseURL(feedURL)
	if err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", feedURL, err)
	}
	items := make([]Item, 0, len(parsed.Items))
	for _, fi := range parsed.Items {
		guid := fi.GUID
		if guid == "" {
			guid = fi.Link
		}
		if guid == "" {
			// skip items with no guid and no link — can't deduplicate or submit
			continue
		}
		items = append(items, Item{
			GUID:        guid,
			URL:         fi.Link,
			Title:       fi.Title,
			PublishedAt: fi.PublishedParsed,
		})
	}
	return items, nil
}
