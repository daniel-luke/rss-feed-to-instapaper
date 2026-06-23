package main

import (
	"log"
	"os"
	"sort"

	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/config"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/feed"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/instapaper"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/state"
)

type pendingItem struct {
	item      feed.Item
	feedLabel string
}

func sortPending(items []pendingItem) {
	sort.Slice(items, func(i, j int) bool {
		pi, pj := items[i].item.PublishedAt, items[j].item.PublishedAt
		if pi == nil && pj == nil {
			return false
		}
		if pi == nil {
			return true
		}
		if pj == nil {
			return false
		}
		return pi.Before(*pj)
	})
}

func main() {
	configPath := envOr("CONFIG_PATH", "/config/config.yaml")
	statePath := envOr("STATE_PATH", "/data/state.db")
	username := requireEnv("INSTAPAPER_USERNAME")
	password := requireEnv("INSTAPAPER_PASSWORD")
	consumerKey := requireEnv("INSTAPAPER_CONSUMER_KEY")
	consumerSecret := requireEnv("INSTAPAPER_CONSUMER_SECRET")

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := state.Open(statePath)
	if err != nil {
		log.Fatalf("open state db: %v", err)
	}
	defer db.Close()

	client := instapaper.NewClient(consumerKey, consumerSecret, username, password)
	if err := client.Authenticate(); err != nil {
		log.Fatalf("authenticate with instapaper: %v", err)
	}

	var pending []pendingItem
	for _, f := range cfg.Feeds {
		if !f.IsEnabled() {
			log.Printf("[%s] skipped (disabled)", f.Label)
			continue
		}
		before := len(pending)
		items, err := feed.FetchItems(f.URL)
		if err != nil {
			log.Printf("ERROR fetch feed %s: %v", f.URL, err)
			continue
		}
		isNew, err := db.RegisterFeedIfNew(f.URL)
		if err != nil {
			log.Printf("ERROR register feed %s: %v", f.URL, err)
		}
		var feedNew []feed.Item
		for _, item := range items {
			sent, err := db.IsSent(item.GUID)
			if err != nil {
				log.Printf("ERROR check state for %s: %v", item.GUID, err)
				continue
			}
			if sent {
				continue
			}
			feedNew = append(feedNew, item)
		}
		if cfg.MaxInitialItems > 0 && isNew && len(feedNew) > cfg.MaxInitialItems {
			feedNew = feedNew[:cfg.MaxInitialItems]
		}
		for _, item := range feedNew {
			pending = append(pending, pendingItem{item: item, feedLabel: f.Label})
		}
		log.Printf("[%s] %d new articles", f.Label, len(pending)-before)
	}

	if !cfg.DisableDateSort {
		sortPending(pending)
	}

	var added int
	for _, p := range pending {
		log.Printf("adding [%s] %q", p.feedLabel, p.item.Title)
		bookmarkID, err := client.Add(p.item.URL, p.item.Title)
		if err != nil {
			log.Printf("ERROR add to instapaper %s: %v", p.item.URL, err)
			continue
		}
		if err := db.MarkSentWithID(p.item.GUID, bookmarkID); err != nil {
			log.Printf("ERROR mark sent %s: %v", p.item.GUID, err)
		}
		added++
	}

	var archived int
	aged, err := db.OldItems(cfg.MaxAgeDays)
	if err != nil {
		log.Printf("ERROR query old items: %v", err)
	} else {
		for _, item := range aged {
			if item.BookmarkID != nil {
				log.Printf("archiving %q (bookmark %d)", item.GUID, *item.BookmarkID)
				if err := client.Archive(*item.BookmarkID); err != nil {
					log.Printf("ERROR archive bookmark %d (%s): %v", *item.BookmarkID, item.GUID, err)
				}
				if err := db.MarkArchived(item.GUID); err != nil {
					log.Printf("ERROR mark archived %s: %v", item.GUID, err)
				}
			} else {
				if err := db.DeleteItem(item.GUID); err != nil {
					log.Printf("ERROR delete item %s: %v", item.GUID, err)
				}
			}
			archived++
		}
	}

	var deleted int
	var toDelete []state.SentItem
	var toDeleteErr error
	if cfg.ClearArchiveOnSync {
		toDelete, toDeleteErr = db.AllArchivedItems()
	} else if cfg.ArchiveRetentionDays > 0 {
		toDelete, toDeleteErr = db.ArchivedItems(cfg.ArchiveRetentionDays)
	}
	if toDeleteErr != nil {
		log.Printf("ERROR query archived items: %v", toDeleteErr)
	} else {
		for _, item := range toDelete {
			if item.BookmarkID != nil {
				log.Printf("deleting %q (bookmark %d)", item.GUID, *item.BookmarkID)
				if err := client.Delete(*item.BookmarkID); err != nil {
					log.Printf("ERROR delete bookmark %d (%s): %v", *item.BookmarkID, item.GUID, err)
				}
			}
			if err := db.DeleteItem(item.GUID); err != nil {
				log.Printf("ERROR delete item %s: %v", item.GUID, err)
			}
			deleted++
		}
	}

	log.Printf("sync complete: %d added, %d archived, %d deleted", added, archived, deleted)
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
