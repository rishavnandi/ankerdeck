package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Refresher struct {
	store    *Store
	database *snapshotDB
	fetcher  *Fetcher
	compat   *CompatClient
	interval time.Duration
	log      *slog.Logger
	cache    *compatCache
	mu       sync.Mutex
	ctxMu    sync.RWMutex
	ctx      context.Context
	manualWG sync.WaitGroup
}

func NewRefresher(store *Store, database *snapshotDB, fetcher *Fetcher, compat *CompatClient, interval time.Duration, log *slog.Logger) *Refresher {
	r := &Refresher{
		store:    store,
		database: database,
		fetcher:  fetcher,
		compat:   compat,
		interval: interval,
		log:      log,
		cache:    newCompatCache(),
	}
	// Restore fresh results without extending their original expiry.
	snapshot := store.Get()
	for _, list := range [][]Game{snapshot.Top, snapshot.Trending} {
		for _, game := range list {
			c := game.Compat
			if c.Checked && c.MatchVersion == compatMatchVersion && !c.CheckedAt.IsZero() && time.Since(c.CheckedAt) < 24*time.Hour {
				r.cache.entries[normalizeTitle(game.Title)] = compatEntry{compat: c, at: c.CheckedAt}
			}
		}
	}
	return r
}

func (r *Refresher) Run(ctx context.Context) {
	r.ctxMu.Lock()
	r.ctx = ctx
	r.ctxMu.Unlock()
	defer r.manualWG.Wait()
	r.Refresh(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Refresh(ctx)
		}
	}
}

func (r *Refresher) Refresh(parent context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := context.WithTimeout(parent, 4*time.Minute)
	defer cancel()

	start := time.Now()

	type result struct {
		games []Game
		err   error
	}
	topCh := make(chan result, 1)
	trendingCh := make(chan result, 1)
	go func() {
		games, err := r.fetcher.Top(ctx)
		topCh <- result{games: games, err: err}
	}()
	go func() {
		games, err := r.fetcher.Trending(ctx)
		trendingCh <- result{games: games, err: err}
	}()

	topResult := <-topCh
	trendingResult := <-trendingCh

	var errs []string
	if topResult.err != nil {
		errs = append(errs, "top games: "+topResult.err.Error())
	}
	if trendingResult.err != nil {
		errs = append(errs, "trending: "+trendingResult.err.Error())
	}

	old := r.store.Get()
	top, trending := old.Top, old.Trending
	updated := false
	if topResult.err == nil && len(topResult.games) > 0 {
		top = topResult.games
		updated = true
	}
	if trendingResult.err == nil && len(trendingResult.games) > 0 {
		trending = trendingResult.games
		updated = true
	}

	r.store.SetBoards(top, trending, errs, updated)
	r.store.SetBoardTimes(topResult.err == nil, trendingResult.err == nil)
	if len(errs) > 0 {
		r.log.Warn("refresh incomplete", "errors", errs, "took", time.Since(start))
	} else {
		r.log.Info("refresh complete", "top", len(top), "trending", len(trending), "took", time.Since(start))
	}

	// Save charts before potentially slow compatibility checks.
	r.save(parent)
	compatCtx, compatCancel := context.WithTimeout(parent, 6*time.Minute)
	defer compatCancel()
	r.enrichCompat(compatCtx)
	r.save(parent)
}

func (r *Refresher) RequestManual(ctx context.Context) (time.Duration, error) {
	retry, err := r.database.takeManualSlot(ctx, time.Now(), 5*time.Minute)
	if err != nil || retry > 0 {
		return retry, err
	}
	r.ctxMu.Lock()
	parent := r.ctx
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		r.ctxMu.Unlock()
		return 0, context.Canceled
	}
	r.manualWG.Add(1)
	r.ctxMu.Unlock()
	go func() {
		defer r.manualWG.Done()
		r.Refresh(parent)
	}()
	return 0, nil
}

func (r *Refresher) save(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := r.database.save(ctx, r.store.Get()); err != nil {
		r.log.Error("save snapshot", "err", err)
	}
}

func (r *Refresher) enrichCompat(ctx context.Context) {
	snapshot := r.store.Get()
	seen := make(map[string]bool)
	// Alternate the charts in rank order so visible games are checked first.
	ordered := make([]Game, 0, len(snapshot.Top)+len(snapshot.Trending))
	for i := 0; i < len(snapshot.Top) || i < len(snapshot.Trending); i++ {
		for _, list := range [][]Game{snapshot.Top, snapshot.Trending} {
			if i >= len(list) {
				continue
			}
			game := list[i]
			key := normalizeTitle(game.Title)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			ordered = append(ordered, game)
		}
	}
	total, done := len(ordered), 0
	pending := make([]Game, 0, total)
	for _, game := range ordered {
		if cached, ok := r.cache.get(normalizeTitle(game.Title)); ok {
			r.store.SetCompat(game.Title, cached)
			done++
		} else {
			pending = append(pending, game)
		}
	}
	r.store.SetProgress(done, total, len(pending) > 0)
	jobs := make(chan Game)
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	for worker := 0; worker < 2; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for game := range jobs {
				if ctx.Err() != nil {
					return
				}
				lookupCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
				result, err := r.compat.Lookup(lookupCtx, game.Title)
				cancel()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					r.log.Warn("compatibility lookup failed", "game", game.Title, "err", err)
					result = Compat{Status: "error", Label: "Unavailable", URL: compatSearchURL(game.Title), Tooltip: "Rating temporarily unavailable. Open CodeWeavers to check this game."}
				} else {
					r.cache.put(normalizeTitle(game.Title), result)
				}
				r.store.SetCompat(game.Title, result)
				progressMu.Lock()
				done++
				r.store.SetProgress(done, total, done < total)
				progressMu.Unlock()
				timer := time.NewTimer(250 * time.Millisecond)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}()
	}
feed:
	for _, game := range pending {
		select {
		case <-ctx.Done():
			break feed
		case jobs <- game:
		}
	}
	close(jobs)
	wg.Wait()
	r.store.SetProgress(done, total, false)
}
