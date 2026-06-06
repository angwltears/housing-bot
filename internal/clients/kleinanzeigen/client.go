package kleinanzeigen

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"housingbot/internal/features/tgBot/service"

	"github.com/PuerkitoBio/goquery"
)

type CleinAnzeigen struct {
	ctx context.Context
	log *slog.Logger
	wg  *sync.WaitGroup
	ch  chan []service.Data
	url string
}

func NewCleinAnzeigen(ctx context.Context, l *slog.Logger, w *sync.WaitGroup, c chan []service.Data, url string) *CleinAnzeigen {
	return &CleinAnzeigen{
		ctx: ctx,
		log: l,
		wg:  w,
		ch:  c,
		url: url,
	}
}
func (c *CleinAnzeigen) scrape() {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequestWithContext(c.ctx, "GET", c.url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		c.log.Error("failed to fetch url")
		return
	}
	if resp.StatusCode != http.StatusOK {
		c.log.Error("status code is not 200", "status_code", resp.StatusCode)
		resp.Body.Close()
		return
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		c.log.Error("failed to parse response")
		resp.Body.Close()
		return
	}
	resp.Body.Close()
	data := []service.Data{}
	doc.Find(".aditem").Each(func(i int, s *goquery.Selection) {
		adID, exists := s.Attr("data-adid")
		if !exists {
			c.log.Error("failed to extract ad id from response")
			return
		}

		title := s.Find("h2.text-module-begin a").Text()
		link, _ := s.Find("h2.text-module-begin a").Attr("href")
		fullLink := "https://www.kleinanzeigen.de" + link
		d := service.Data{
			URL:   fullLink,
			ID:    "kleinanzeigen:" + strings.TrimSpace(adID),
			Title: title,
		}
		data = append(data, d)

	})
	if len(data) > 0 {
		c.ch <- data
	}
}
func (c *CleinAnzeigen) Process() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	defer c.wg.Done()
	c.scrape()
	for {
		select {
		case <-c.ctx.Done():
			c.log.Error("finished kleinanzeigen service")
			return
		case <-ticker.C:
			c.scrape()
		}

	}
}
