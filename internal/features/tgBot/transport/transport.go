package transport

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"housingbot/internal/features/tgBot/service"

	"gopkg.in/telebot.v3"
)

type Transport struct {
	ctx    context.Context
	bot    *telebot.Bot
	logger *slog.Logger
	ID     []int64
	ch     <-chan service.Data
	wg     *sync.WaitGroup
}

func NewTransport(ctx context.Context, wg *sync.WaitGroup, bot *telebot.Bot, logger *slog.Logger, ID []int64, ch <-chan service.Data) *Transport {
	return &Transport{
		ctx:    ctx,
		bot:    bot,
		logger: logger,
		ID:     ID,
		ch:     ch,
		wg:     wg,
	}
}
func (t *Transport) SetupCallbacks() {
	t.bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}
		data := cb.Data
		parts := strings.SplitN(data, ":", 2)
		if len(parts) < 2 {
			return c.Respond()
		}
		action := parts[0]
		id := parts[1]

		original := cb.Message
		if original == nil {
			return c.Respond()
		}

		switch action {
		case "hide":
			// prefer Text, fallback to Caption
			body := strings.TrimSpace(original.Text)
			if body == "" {
				body = strings.TrimSpace(original.Caption)
			}
			spoilered := "<tg-spoiler>" + body + "</tg-spoiler>"
			newText := spoilered + "\n\n" + "✅ <b>CHECKED</b>"

			// extract URL using helper
			url := t.getURLFromMessage(original)
			if url == "" {
				// log missing URL
				t.logger.Debug("no url found in message", "advert_id", id)
			}

			menu := &telebot.ReplyMarkup{}
			if url != "" {
				openBtn := menu.URL("Open", url)
				menu.Inline(menu.Row(openBtn))
			} else {
				// no URL found - remove inline keyboard
				menu = &telebot.ReplyMarkup{}
			}

			_, err := t.bot.Edit(original, newText, telebot.ModeHTML, menu)
			if err != nil {
				t.logger.Error("failed to edit message", "err", err, "advert_id", id)
			}
			return c.Respond(&telebot.CallbackResponse{Text: "Marked checked"})
		default:
			return c.Respond()
		}
	})
}

func (t *Transport) getURLFromMessage(m *telebot.Message) string {
	if m == nil {
		return ""
	}

	// try entities in text
	if len(m.Entities) > 0 && len(m.Text) > 0 {
		for _, e := range m.Entities {
			if e.Type == "text_link" && e.URL != "" {
				return e.URL
			}
			if e.Type == "url" {
				start := e.Offset
				end := e.Offset + e.Length
				if start >= 0 && end <= len(m.Text) && start < end {
					return m.Text[start:end]
				}
			}
		}
	}

	// try entities in caption
	if len(m.CaptionEntities) > 0 && len(m.Caption) > 0 {
		for _, e := range m.CaptionEntities {
			if e.Type == "text_link" && e.URL != "" {
				return e.URL
			}
			if e.Type == "url" {
				start := e.Offset
				end := e.Offset + e.Length
				if start >= 0 && end <= len(m.Caption) && start < end {
					return m.Caption[start:end]
				}
			}
		}
	}

	return ""
}

func (t *Transport) Process() {
	defer t.wg.Done()
	for {
		select {
		case <-t.ctx.Done():
			return
		case data := <-t.ch:
			msg := fmt.Sprintf("🏠 <b>%s</b>\n\n🔗 <a href=\"%s\">Смотреть объявление</a>", data.Title, data.URL)
			menu := &telebot.ReplyMarkup{}
			openBtn := menu.URL("Open", data.URL)
			hideBtn := menu.Data("🗑 Hide", fmt.Sprintf("hide:%s", data.ID))
			menu.Inline(
				menu.Row(openBtn, hideBtn),
			)

			for _, id := range t.ID {
				user := &telebot.User{ID: id}
				t.logger.Debug("sending message to user", "user_id", id, "advert_id", data.ID)
				_, err := t.bot.Send(user, msg, telebot.ModeHTML, telebot.NoPreview, menu)
				if err != nil {
					t.logger.Error("failed to send message", "err", err)
					continue
				}
			}
		}
	}
}
