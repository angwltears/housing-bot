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
	ctx       context.Context
	bot       *telebot.Bot
	logger    *slog.Logger
	ID        []int64
	IDMunchen []int64
	ch        <-chan service.Data
	wg        *sync.WaitGroup
}

func NewTransport(ctx context.Context, wg *sync.WaitGroup, bot *telebot.Bot, logger *slog.Logger, ID []int64, IDMunchen []int64, ch <-chan service.Data) *Transport {
	return &Transport{
		ctx:       ctx,
		bot:       bot,
		logger:    logger,
		ID:        ID,
		ch:        ch,
		wg:        wg,
		IDMunchen: IDMunchen,
	}
}

func (t *Transport) SetupCallbacks() {
	t.bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		cb := c.Callback()
		if cb == nil {
			return nil
		}
		data := strings.TrimSpace(strings.TrimLeftFunc(cb.Data, func(r rune) bool { return r < ' ' }))
		if data == "" {
			return c.Respond()
		}
		parts := strings.SplitN(data, ":", 2)
		if len(parts) < 2 || parts[0] != "hide" {
			return c.Respond()
		}
		id := parts[1]
		original := cb.Message
		if original == nil {
			return c.Respond()
		}
		url := t.getURLFromMessage(original)
		raw := original.Text
		if raw == "" {
			raw = original.Caption
		}
		title := strings.SplitN(raw, "\n\n", 2)[0]
		newText := fmt.Sprintf("<tg-spoiler>%s</tg-spoiler>\n\n✅ <b>CHECKED</b>", strings.TrimSpace(title))
		menu := &telebot.ReplyMarkup{}
		if url != "" {
			menu.Inline(menu.Row(menu.URL("Open", url)))
		}

		if _, err := t.bot.Edit(original, newText, telebot.ModeHTML, menu); err != nil {
			t.logger.Error("failed to edit message", "err", err, "advert_id", id)
		}

		return c.Respond(&telebot.CallbackResponse{Text: "Marked checked"})
	})
}
func (t *Transport) getURLFromMessage(m *telebot.Message) string {
	if m == nil {
		return ""
	}
	entities := append(m.Entities, m.CaptionEntities...)
	for _, e := range entities {
		if e.Type == "text_link" && e.URL != "" {
			return e.URL
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
			menu.Inline(menu.Row(menu.Data("🗑 Hide", fmt.Sprintf("hide:%s", data.ID))))

			for _, id := range t.ID {
				user := &telebot.User{ID: id}
				if _, err := t.bot.Send(user, msg, telebot.ModeHTML, telebot.NoPreview, menu); err != nil {
					t.logger.Error("failed to send message", "err", err, "advert_id", data.ID)
				}
			}

		}
	}
}
