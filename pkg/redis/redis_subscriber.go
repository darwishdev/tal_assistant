package redis

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

type SignalEvent struct {
	SessionID string `json:"session_id"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

type NQIEvent struct {
	SessionID    string `json:"session_id"`
	NextQuestion string `json:"next_question"`
	Rationale    string `json:"rationale"`
}

type RedisSubscriber struct {
	client   *redis.Client
	nqiCh    string
	signalCh string
	OnSignal func(SignalEvent)
	OnNQI    func(NQIEvent)
}

func NewRedisSubscriber(onSignal func(SignalEvent), onNQI func(NQIEvent)) *RedisSubscriber {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6378"
	}
	nqiCh := os.Getenv("NQI_CHANNEL")
	if nqiCh == "" {
		nqiCh = "nqi:results"
	}
	signalCh := os.Getenv("SIGNAL_CHANNEL")
	if signalCh == "" {
		signalCh = "signal:results"
	}

	client := redis.NewClient(&redis.Options{Addr: addr})

	return &RedisSubscriber{
		client:   client,
		nqiCh:    nqiCh,
		signalCh: signalCh,
		OnSignal: onSignal,
		OnNQI:    onNQI,
	}
}

// Run blocks until ctx is cancelled. Call it in a goroutine.
func (s *RedisSubscriber) Run(ctx context.Context) {
	if err := s.client.Ping(ctx).Err(); err != nil {
		log.Printf("[redis-sub] ping failed: %v", err)
		return
	}

	pubsub := s.client.Subscribe(ctx, s.nqiCh, s.signalCh)
	defer pubsub.Close()

	log.Printf("[redis-sub] subscribed — signal=%s nqi=%s", s.signalCh, s.nqiCh)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			log.Println("[redis-sub] context cancelled, stopping")
			return
		case msg, ok := <-ch:
			if !ok {
				log.Println("[redis-sub] channel closed")
				return
			}
			s.dispatch(msg)
		}
	}
}
func (s *RedisSubscriber) dispatch(msg *redis.Message) {
	switch msg.Channel {
	case s.signalCh:
		var e SignalEvent
		if err := json.Unmarshal([]byte(msg.Payload), &e); err != nil {
			log.Printf("[redis-sub] bad signal payload: %v", err)
			return
		}
		if s.OnSignal != nil {
			s.OnSignal(e)
		}

	case s.nqiCh:
		var e NQIEvent
		if err := json.Unmarshal([]byte(msg.Payload), &e); err != nil {
			log.Printf("[redis-sub] bad nqi payload: %v", err)
			return
		}
		// strip the nqi- prefix added by the signal detector
		e.SessionID = strings.TrimPrefix(e.SessionID, "nqi-")
		if s.OnNQI != nil {
			s.OnNQI(e)
		}
	}
}
func (s *RedisSubscriber) Close() error {
	return s.client.Close()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
