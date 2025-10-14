package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// KafkaMessaging handles Kafka-based message passing
type KafkaMessaging struct {
	config  *types.Config
	logger  *zap.Logger
	writers map[string]*kafka.Writer
	readers map[string]*kafka.Reader
}

// NewKafkaMessaging creates a new Kafka messaging system
func NewKafkaMessaging(config *types.Config, logger *zap.Logger) *KafkaMessaging {
	return &KafkaMessaging{
		config:  config,
		logger:  logger,
		writers: make(map[string]*kafka.Writer),
		readers: make(map[string]*kafka.Reader),
	}
}

// GetWriter gets or creates a Kafka writer for a topic
func (km *KafkaMessaging) GetWriter(topic string) *kafka.Writer {
	fullTopic := km.config.KafkaTopicPrefix + "." + topic

	if writer, exists := km.writers[fullTopic]; exists {
		return writer
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(km.config.KafkaBrokers...),
		Topic:        fullTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
	}

	km.writers[fullTopic] = writer
	km.logger.Info("Created Kafka writer", zap.String("topic", fullTopic))

	return writer
}

// GetReader gets or creates a Kafka reader for a topic
func (km *KafkaMessaging) GetReader(topic, groupID string) *kafka.Reader {
	fullTopic := km.config.KafkaTopicPrefix + "." + topic

	key := fullTopic + ":" + groupID
	if reader, exists := km.readers[key]; exists {
		return reader
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        km.config.KafkaBrokers,
		Topic:          fullTopic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})

	km.readers[key] = reader
	km.logger.Info("Created Kafka reader",
		zap.String("topic", fullTopic),
		zap.String("group_id", groupID),
	)

	return reader
}

// PublishMessage publishes a message to a topic
func (km *KafkaMessaging) PublishMessage(ctx context.Context, topic string, message *types.Message) error {
	writer := km.GetWriter(topic)

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(message.ID),
		Value: data,
		Time:  message.Timestamp,
	})

	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	km.logger.Debug("Published message",
		zap.String("topic", topic),
		zap.String("message_id", message.ID),
		zap.String("type", string(message.Type)),
	)

	return nil
}

// ConsumeMessages consumes messages from a topic
func (km *KafkaMessaging) ConsumeMessages(ctx context.Context, topic, groupID string, handler func(*types.Message) error) error {
	reader := km.GetReader(topic, groupID)
	defer reader.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				km.logger.Error("Failed to read message", zap.Error(err))
				continue
			}

			var message types.Message
			if err := json.Unmarshal(msg.Value, &message); err != nil {
				km.logger.Error("Failed to unmarshal message", zap.Error(err))
				continue
			}

			if err := handler(&message); err != nil {
				km.logger.Error("Failed to handle message",
					zap.Error(err),
					zap.String("message_id", message.ID),
				)
			}
		}
	}
}

// PublishInsight publishes an insight to the knowledge mesh
func (km *KafkaMessaging) PublishInsight(ctx context.Context, insight *types.Insight) error {
	// Wrap insight in a message
	message := &types.Message{
		ID:          string(insight.ID),
		FromAgentID: insight.AgentID,
		Type:        "insight",
		Payload: map[string]any{
			"insight": insight,
		},
		Timestamp: insight.CreatedAt,
	}

	return km.PublishMessage(ctx, "insights", message)
}

// PublishTopologyEvent publishes a topology event
func (km *KafkaMessaging) PublishTopologyEvent(ctx context.Context, event types.TopologyEvent) error {
	writer := km.GetWriter("topology")

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(string(event.Type)),
		Value: data,
		Time:  event.Timestamp,
	})

	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

// ConsumeTopologyEvents consumes topology events from a topic
func (km *KafkaMessaging) ConsumeTopologyEvents(ctx context.Context, topic, groupID string, handler func(types.TopologyEvent) error) error {
	reader := km.GetReader(topic, groupID)
	defer reader.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				km.logger.Error("Failed to read message", zap.Error(err))
				continue
			}

			var event types.TopologyEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				km.logger.Error("Failed to unmarshal topology event", zap.Error(err))
				continue
			}

			if err := handler(event); err != nil {
				km.logger.Error("Failed to handle topology event",
					zap.Error(err),
					zap.String("event_type", string(event.Type)),
				)
			}
		}
	}
}

// PublishProposal publishes a consensus proposal
func (km *KafkaMessaging) PublishProposal(ctx context.Context, proposal *types.Proposal) error {
	writer := km.GetWriter("proposals")

	data, err := json.Marshal(proposal)
	if err != nil {
		return fmt.Errorf("failed to marshal proposal: %w", err)
	}

	err = writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(string(proposal.ID)),
		Value: data,
		Time:  proposal.CreatedAt,
	})

	if err != nil {
		return fmt.Errorf("failed to write proposal: %w", err)
	}

	return nil
}

// Close closes all Kafka connections
func (km *KafkaMessaging) Close() error {
	for topic, writer := range km.writers {
		if err := writer.Close(); err != nil {
			km.logger.Error("Failed to close writer", zap.String("topic", topic), zap.Error(err))
		}
	}

	for key, reader := range km.readers {
		if err := reader.Close(); err != nil {
			km.logger.Error("Failed to close reader", zap.String("key", key), zap.Error(err))
		}
	}

	km.logger.Info("Kafka messaging closed")
	return nil
}
