package service

import (
	"context"
	"log/slog"
	"sync"
)

type SessionCache interface {
	AddIfNotExists(ctx context.Context, id string) (bool, error)
}
type Data struct {
	URL   string
	ID    string
	Title string
}
type Service struct {
	ctx    context.Context
	logger *slog.Logger
	wg     *sync.WaitGroup
	ch     <-chan []Data
	tchan  chan Data
	cache  SessionCache
}

func NewService(ctx context.Context, logger *slog.Logger, wg *sync.WaitGroup, ch <-chan []Data, tchan chan Data, cache SessionCache) *Service {
	return &Service{
		ctx:    ctx,
		wg:     wg,
		ch:     ch,
		logger: logger,
		tchan:  tchan,
		cache:  cache,
	}
}
func (s *Service) Process() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Error("shutting down service")

			return
		case data := <-s.ch:
			for _, d := range data {
				added, err := s.cache.AddIfNotExists(s.ctx, d.ID)
				if err != nil {
					s.logger.Error("failed to check advert uniqueness", "id", d.ID, "err", err)
					continue
				}
				if !added {
					continue
				}
				s.tchan <- d
			}
		}
	}
}
