package feed

import (
	"fmt"

	"github.com/mmcdole/gofeed"
)

type Item struct {
	GUID  string
	URL   string
	Title string
}

func FetchItems(feedURL string) ([]Item, error) {
	fp := gofeed.NewParser()
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
		items = append(items, Item{
			GUID:  guid,
			URL:   fi.Link,
			Title: fi.Title,
		})
	}
	return items, nil
}
