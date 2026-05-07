// Package events publishes CloudEvents 1.0 activity events from
// tas-agent-builder to Kafka for the TAS Live Streams pipeline.
package events

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"

	tasevents "github.com/Tributary-ai-services/aether-shared/go-events"
	"github.com/Tributary-ai-services/aether-shared/go-events/kafkabind"
	"github.com/Tributary-ai-services/aether-shared/go-events/payloads"
	"github.com/Tributary-ai-services/aether-shared/go-events/topics"
)

const ceSource = "urn:tas:service:agent-builder"

type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// Publisher fire-and-forget publishes agent activity events. A nil receiver
// is safe — every method becomes a no-op so callers can construct without
// Kafka and skip nil checks at every call site.
type Publisher struct {
	writer  kafkaWriter
	logger  *log.Logger
	timeout time.Duration
}

// Config configures the Publisher.
type Config struct {
	Brokers      []string
	Topic        string
	Logger       *log.Logger
	WriteTimeout time.Duration
}

// New returns a Publisher backed by a kafka-go Writer. Returns nil if
// brokers is empty so callers can pass the result straight through.
func New(cfg Config) *Publisher {
	if len(cfg.Brokers) == 0 {
		return nil
	}
	topic := cfg.Topic
	if topic == "" {
		topic = topics.ActivityAgents
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	timeout := cfg.WriteTimeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: timeout,
		RequiredAcks: kafka.RequireOne,
	}
	return &Publisher{writer: writer, logger: logger, timeout: timeout}
}

// NewWithWriter is for tests.
func NewWithWriter(w kafkaWriter, logger *log.Logger) *Publisher {
	if logger == nil {
		logger = log.Default()
	}
	return &Publisher{writer: w, logger: logger, timeout: 2 * time.Second}
}

// PublishCreated emits com.tas.activity.agent.created.
func (p *Publisher) PublishCreated(ctx context.Context, tenantID, userID, requestID string, payload payloads.AgentCreated) {
	p.publish(ctx, payloads.TypeAgentCreated, tenantID, userID, requestID, payload.AgentID, tasevents.SeverityInfo, payload)
}

// PublishExecuted emits com.tas.activity.agent.executed.
func (p *Publisher) PublishExecuted(ctx context.Context, tenantID, userID, requestID string, payload payloads.AgentExecuted) {
	p.publish(ctx, payloads.TypeAgentExecuted, tenantID, userID, requestID, payload.AgentID, tasevents.SeverityInfo, payload)
}

// PublishFailed emits com.tas.activity.agent.failed.
func (p *Publisher) PublishFailed(ctx context.Context, tenantID, userID, requestID string, payload payloads.AgentFailed) {
	p.publish(ctx, payloads.TypeAgentFailed, tenantID, userID, requestID, payload.AgentID, tasevents.SeverityHigh, payload)
}

func (p *Publisher) publish(ctx context.Context, eventType, tenantID, userID, requestID, subject string, sev tasevents.Severity, data any) {
	if p == nil || p.writer == nil {
		return
	}

	ce := tasevents.New(eventType, ceSource,
		tasevents.WithTenant(tenantID, ""),
		tasevents.WithUser(userID),
		tasevents.WithRequest(requestID),
		tasevents.WithSubject(subject),
		tasevents.WithSeverity(sev),
		tasevents.WithData(data),
	)

	value, headers, err := kafkabind.Encode(ce)
	if err != nil {
		p.logger.Printf("agent-builder events: encode %s failed: %v", eventType, err)
		return
	}

	writeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	msg := kafka.Message{
		Key:     kafkabind.MessageKey(ce),
		Value:   value,
		Headers: headers,
		Time:    ce.Time,
	}
	if err := p.writer.WriteMessages(writeCtx, msg); err != nil {
		p.logger.Printf("agent-builder events: publish %s failed: %v", eventType, err)
	}
}

// Close flushes pending messages.
func (p *Publisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
