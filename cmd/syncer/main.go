package main

import (
	"log"
	"os"

	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/config"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/feed"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/instapaper"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/state"
)

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

	for _, f := range cfg.Feeds {
		log.Printf("processing feed: %s (%s)", f.Label, f.URL)
		items, err := feed.FetchItems(f.URL)
		if err != nil {
			log.Printf("ERROR fetch feed %s: %v", f.URL, err)
			continue
		}

		for _, item := range items {
			sent, err := db.IsSent(item.GUID)
			if err != nil {
				log.Printf("ERROR check state for %s: %v", item.GUID, err)
				continue
			}
			if sent {
				continue
			}

			bookmarkID, err := client.Add(item.URL, item.Title)
			if err != nil {
				log.Printf("ERROR add to instapaper %s: %v", item.URL, err)
				continue
			}

			if err := db.MarkSentWithID(item.GUID, bookmarkID); err != nil {
				log.Printf("ERROR mark sent %s: %v", item.GUID, err)
			}
		}
	}

	aged, err := db.OldItems(cfg.MaxAgeDays)
	if err != nil {
		log.Printf("ERROR query old items: %v", err)
	} else {
		for _, item := range aged {
			if item.BookmarkID != nil {
				if err := client.Archive(*item.BookmarkID); err != nil {
					log.Printf("ERROR archive bookmark %d (%s): %v", *item.BookmarkID, item.GUID, err)
				}
			}
			if err := db.DeleteItem(item.GUID); err != nil {
				log.Printf("ERROR delete item %s: %v", item.GUID, err)
			}
		}
	}

	log.Printf("sync complete")
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
