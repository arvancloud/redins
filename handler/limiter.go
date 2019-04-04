package handler

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type RateLimiter struct {
	Limiters  *cache.Cache
	MaxTime   time.Duration
	TimeStep  time.Duration
	Config    *RateLimiterConfig
	WhiteList map[string]interface{}
	BlackList map[string]interface{}
}

type RateLimiterConfig struct {
	Enable    bool     `json:"enable"`
	Burst     int      `json:"burst"`
	Rate      int      `json:"rate"`
	WhiteList []string `json:"whitelist"`
	BlackList []string `json:"blacklist"`
}

func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		Config: config,
	}
	rl.Limiters = cache.New(time.Minute, time.Minute*10)
	rl.WhiteList = make(map[string]interface{})
	for _, x := range config.WhiteList {
		rl.WhiteList[x] = nil
	}
	rl.BlackList = make(map[string]interface{})
	for _, x := range config.BlackList {
		rl.BlackList[x] = nil
	}
	rl.TimeStep = time.Duration(60000/config.Rate) * time.Millisecond
	rl.MaxTime = rl.TimeStep * time.Duration(config.Burst)
	return rl
}

type Limiter struct {
	Size       time.Duration
	LastUpdate time.Time
	Mutex      *sync.Mutex
}

func (rl *RateLimiter) CanHandle(key string) bool {
	if !rl.Config.Enable {
		return true
	}

	if _, exist := rl.BlackList[key]; exist {
		return false
	}
	if _, exist := rl.WhiteList[key]; exist {
		return true
	}
	var (
		res bool
		l   *Limiter
	)
	value, found := rl.Limiters.Get(key)
	if found {
		l = value.(*Limiter)
		l.Mutex.Lock()

		l.Size -= time.Since(l.LastUpdate)
		l.LastUpdate = time.Now()

		if l.Size < 0 {
			l.Size = 0
		}
		if l.Size > rl.MaxTime {
			res = false
		} else {
			l.Size += rl.TimeStep
			res = true
		}
		rl.Limiters.Set(key, l, time.Minute)
		l.Mutex.Unlock()
	} else {
		l = &Limiter{
			Size:       rl.TimeStep,
			LastUpdate: time.Now(),
			Mutex:      &sync.Mutex{},
		}
		res = true
		rl.Limiters.Set(key, l, time.Minute)
	}
	return res
}
