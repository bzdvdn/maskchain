package profilerepo

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"
)

// @sk-task 102-profile-cache#T3.2: Implement PubSub subscriber goroutine (RQ-006, RQ-012)
type PubSubSubscriber struct {
	client      valkey.Client
	invalidated *InvalidationTracker
	metrics     cacheMetrics
	logger      *slog.Logger
	done        chan struct{}
}

func NewPubSubSubscriber(client valkey.Client, invalidated *InvalidationTracker, metrics cacheMetrics, logger *slog.Logger) *PubSubSubscriber {
	return &PubSubSubscriber{
		client:      client,
		invalidated: invalidated,
		metrics:     metrics,
		logger:      logger,
		done:        make(chan struct{}),
	}
}

func (s *PubSubSubscriber) Start() {
	go s.run()
}

func (s *PubSubSubscriber) Stop() {
	close(s.done)
}

const pubSubPattern = "profile.invalidate:*"

func (s *PubSubSubscriber) run() {
	for {
		select {
		case <-s.done:
			return
		default:
		}

		if err := s.subscribeOnce(); err != nil {
			s.logger.Warn("pubsub subscriber disconnected, reconnecting", "error", err)
			select {
			case <-s.done:
				return
			case <-time.After(time.Second):
			}
		}
	}
}

func (s *PubSubSubscriber) subscribeOnce() error {
	if s.client == nil {
		<-s.done
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-s.done:
			cancel()
		case <-ctx.Done():
		}
	}()

	s.logger.Info("pubsub subscriber connecting", "pattern", pubSubPattern)
	return s.client.Receive(ctx,
		s.client.B().Psubscribe().Pattern(pubSubPattern).Build(),
		func(msg valkey.PubSubMessage) {
			s.handleMessage(msg)
		},
	)
}

func (s *PubSubSubscriber) handleMessage(msg valkey.PubSubMessage) {
	slug := strings.TrimPrefix(msg.Channel, "profile.invalidate:")
	if slug == "" || slug == msg.Channel {
		s.logger.Warn("pubsub received unexpected channel", "channel", msg.Channel)
		return
	}
	if s.invalidated != nil {
		s.invalidated.Add(slug)
	}
	s.metrics.IncInvalidations("pubsub")
}
