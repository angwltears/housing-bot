package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"housingbot/internal/clients/kleinanzeigen"
	redisclient "housingbot/internal/clients/redis"
	"housingbot/internal/clients/wggesucht"
	"housingbot/internal/clients/wohnungsboerse"
	"housingbot/internal/core/config"
	"housingbot/internal/core/logger"
	"housingbot/internal/features/tgBot/service"
	"housingbot/internal/features/tgBot/transport"

	"gopkg.in/telebot.v3"
)

func main() {
	log := logger.SetupLogger("local")
	cfg := config.MustLoad(log)
	wg := new(sync.WaitGroup)
	ctx, cancel := context.WithCancel(context.Background())
	settings := telebot.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}
	bot, err := telebot.NewBot(settings)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
	redisSession, err := redisclient.NewRedisSession(cfg.Redis.URL, 150*time.Hour)
	if err != nil {
		log.Error("failed to create redis session", "err", err)
		os.Exit(1)
	}
	if err := redisSession.PingRedisClient(); err != nil {
		log.Error("failed to ping redis", "err", err)
		os.Exit(1)
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Info("shutting down all services gracefully...")
		cancel()
		bot.Stop()
	}()
	transportChan := make(chan service.Data, 100)
	clientChan := make(chan []service.Data, 100)

	transp := transport.NewTransport(ctx, wg, bot, log, []int64{750791661, 975016573}, []int64{7283992399}, transportChan)
	transp.SetupCallbacks()
	srvc := service.NewService(ctx, log, wg, clientChan, transportChan, redisSession)
	klein := kleinanzeigen.NewCleinAnzeigen(ctx, log, wg, clientChan, cfg.Site.KleinanzeigenURL)
	wggsht := wggesucht.NewWggesucht(ctx, log, wg, clientChan, cfg.Site.WgGeshuchtURL)
	boerse := wohnungsboerse.NewWohnungsboerse(ctx, log, wg, clientChan, cfg.Site.WohnungsBoerseURL)
	wg.Add(1)
	go transp.Process()
	wg.Add(1)
	go srvc.Process()
	wg.Add(1)
	go klein.Process()
	wg.Add(1)
	go wggsht.Process()
	wg.Add(1)
	go boerse.Process()
	bot.Start()
	wg.Wait()

}
