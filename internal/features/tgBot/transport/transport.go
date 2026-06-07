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
		// log raw callback for diagnostics (may contain control chars)
		t.logger.Debug("callback received (raw)", "data", data)
		// strip control characters that may be injected by telebot (eg. '\f')
		data = strings.TrimLeftFunc(data, func(r rune) bool { return r < ' ' })
		data = strings.TrimSpace(data)
		if data == "" {
			_ = c.Respond()
			return nil
		}
		// log sanitized data
		t.logger.Debug("callback received (sanitized)", "data", data)
		parts := strings.SplitN(data, ":", 2)
		if len(parts) < 2 {
			_ = c.Respond()
			return nil
		}
		action := parts[0]
		id := parts[1]

		original := cb.Message
		if original == nil {
			err := c.Respond()
			if err != nil {
				t.logger.Debug("callback respond failed (nil message)", "err", err)
			}
			return nil
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
				// still respond to callback so user doesn't see spinning
				_ = c.Respond(&telebot.CallbackResponse{Text: "Error marking checked"})
				return nil
			}
			// log success
			t.logger.Debug("message edited (marked checked)", "advert_id", id, "message_id", original.ID)
			err = c.Respond(&telebot.CallbackResponse{Text: "Marked checked"})
			if err != nil {
				t.logger.Debug("callback respond failed (success)", "err", err)
			}
			return nil
		default:
			err := c.Respond()
			if err != nil {
				t.logger.Debug("callback respond failed (unknown action)", "err", err)
			}
			return nil
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

func (t *Transport) removeURLFromMessage(m *telebot.Message) string {
	if m == nil {
		return ""
	}

	// try removing url/text_link spans from Text
	if len(m.Entities) > 0 && len(m.Text) > 0 {
		s := m.Text
		var b strings.Builder
		last := 0
		for _, e := range m.Entities {
			if e.Type == "text_link" || e.Type == "url" {
				start := e.Offset
				end := e.Offset + e.Length
				if start < 0 || end > len(s) || start >= end {
					continue
				}
				if last < start {
					b.WriteString(s[last:start])
				}
				last = end
			}
		}
		if last < len(s) {
			b.WriteString(s[last:])
		}
		res := strings.TrimSpace(b.String())
		if res != "" {
			return res
		}
	}

	// try removing url/text_link spans from Caption
	if len(m.CaptionEntities) > 0 && len(m.Caption) > 0 {
		s := m.Caption
		var b strings.Builder
		last := 0
		for _, e := range m.CaptionEntities {
			if e.Type == "text_link" || e.Type == "url" {
				start := e.Offset
				end := e.Offset + e.Length
				if start < 0 || end > len(s) || start >= end {
					continue
				}
				if last < start {
					b.WriteString(s[last:start])
				}
				last = end
			}
		}
		if last < len(s) {
			b.WriteString(s[last:])
		}
		res := strings.TrimSpace(b.String())
		if res != "" {
			return res
		}
	}

	// fallback: remove lines that look like link lines
	text := strings.TrimSpace(m.Text)
	if text == "" {
		text = strings.TrimSpace(m.Caption)
	}
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		if strings.Contains(line, "🔗") || strings.Contains(line, "Смотреть объявление") || strings.Contains(line, "http") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
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
			hideBtn := menu.Data("🗑 Hide", fmt.Sprintf("hide:%s", data.ID))
			menu.Inline(
				menu.Row(hideBtn),
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
