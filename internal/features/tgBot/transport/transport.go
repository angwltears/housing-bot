package transport

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"housingbot/internal/features/tgBot/service"

	"gopkg.in/telebot.v3"
)

type Transport struct {
	ctx  context.Context
	bot  *telebot.Bot
	logger *slog.Logger
	ID   []int64
	ch   <-chan service.Data
	wg   *sync.WaitGroup
	menu *telebot.ReplyMarkup
}

func NewTransport(ctx context.Context, wg *sync.WaitGroup, bot *telebot.Bot, logger *slog.Logger, ID []int64, ch <-chan service.Data) *Transport {
	menu := &telebot.ReplyMarkup{}
	btnLike := menu.Data("👍 Like", "like_flat")
	btnSkip := menu.Data("🗑 Skip", "skip_flat")
	menu.Inline(
		menu.Row(btnLike, btnSkip),
	)

	return &Transport{
		ctx:    ctx,
		bot:    bot,
		logger: logger,
		ID:     ID,
		ch:     ch,
		wg:     wg,
		menu:   menu,
	}
}

func (t *Transport) SetupCallbacks() {
	btnLike := t.menu.InlineKeyboard[0][0]
	btnSkip := t.menu.InlineKeyboard[0][1]

	t.bot.Handle(&btnLike, func(c telebot.Context) error {
		originalMsg := c.Message()
		newText := "✅ <b>MARKED AS CHECKED</b>\n\n" + originalMsg.Text
		err := t.bot.Edit(originalMsg, newText, telebot.ModeHTML, &telebot.ReplyMarkup{})
		if err != nil {
			t.logger.Error("failed to edit message", "err", err)
		}
		return c.Respond(&telebot.CallbackResponse{
			Text: "✅ Added to liked",
		})
	})

	t.bot.Handle(&btnSkip, func(c telebot.Context) error {
		originalMsg := c.Message()
		newText := "❌ <b>SKIPPED</b>\n\n" + originalMsg.Text
		err := t.bot.Edit(originalMsg, newText, telebot.ModeHTML, &telebot.ReplyMarkup{})
		if err != nil {
			t.logger.Error("failed to edit message", "err", err)
		}
		return c.Respond(&telebot.CallbackResponse{
			Text: "❌ Skipped",
		})
	})
}

func (t *Transport) Process() {
	defer t.wg.Done()
	for {
		select {
		case <-t.ctx.Done():
			return
		case data := <-t.ch:
			msg := fmt.Sprintf("🏠 <b>%s</b>\n\n🔗 <a href=\"%s\">Смотреть объявление</a>", data.Title, data.URL)
			for _, id := range t.ID {
				user := &telebot.User{ID: id}
				t.logger.Debug("sending message to user", "user_id", id, "advert_id", data.ID)
				_, err := t.bot.Send(user, msg, telebot.ModeHTML, telebot.NoPreview, t.menu)
				if err != nil {
					t.logger.Error("failed to send message", "err", err)
					continue
				}
			}
		}
	}
}
