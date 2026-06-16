package wohnungsboerse

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"housingbot/internal/features/tgBot/service"

	"github.com/PuerkitoBio/goquery"
	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Wohnungsboerse struct {
	ctx context.Context
	log *slog.Logger
	wg  *sync.WaitGroup
	ch  chan []service.Data
	url string
}

func NewWohnungsboerse(ctx context.Context, l *slog.Logger, w *sync.WaitGroup, c chan []service.Data, url string) *Wohnungsboerse {
	return &Wohnungsboerse{
		ctx: ctx,
		log: l,
		wg:  w,
		ch:  c,
		url: url,
	}
}

func (w *Wohnungsboerse) scrape() {
	// Инициализируем tls-client для обхода защиты сайта
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithNotFollowRedirects(),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewLogger(), options...)
	if err != nil {
		w.log.Error("failed to create tls client", "err", err)
		return
	}

	req, err := http.NewRequestWithContext(w.ctx, "GET", w.url, nil)
	if err != nil {
		w.log.Error("failed to create request", "err", err)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		w.log.Error("failed to fetch url", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		w.log.Error("status code is not 200", "status_code", resp.StatusCode)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		w.log.Error("failed to parse html", "err", err)
		return
	}

	var data []service.Data

	doc.Find("a[href*='/immodetail/']").Each(func(i int, s *goquery.Selection) {
		link, exists := s.Attr("href")
		if !exists {
			return
		}

		parts := strings.Split(link, "/")
		adID := parts[len(parts)-1]

		title := strings.TrimSpace(s.Find("h3").Text())

		if title != "" && adID != "" {
			data = append(data, service.Data{
				URL:   link,
				ID:    "wohnungsboerse:" + adID,
				Title: title,
			})
		}
	})

	if len(data) > 0 {
		w.ch <- data
	}
}

func (w *Wohnungsboerse) Process() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	defer w.wg.Done()

	w.scrape()

	for {
		select {
		case <-w.ctx.Done():
			w.log.Info("finished wohnungsboerse service")
			return
		case <-ticker.C:
			w.scrape()
		}
	}
}
