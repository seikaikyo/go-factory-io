// Package mqtt provides an MQTT bridge that publishes GEM events to an MQTT broker.
// This enables integration with factory MES/SCADA systems via standard MQTT message bus.
package mqtt

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/driver/gem"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// Config holds MQTT bridge configuration.
type Config struct {
	BrokerURL     string        // tcp://host:1883 or ssl://host:8883
	ClientID      string        // MQTT client ID
	TopicPrefix   string        // e.g., "factory/equip01"
	QoS           byte          // 0, 1, or 2
	Username      string        // Optional auth
	Password      string        // Optional auth
	TLSConfig     *tls.Config   // Optional TLS
	KeepAlive     time.Duration // Default 30s
	AutoReconnect bool          // Default true
	CleanSession  bool          // Default true
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(brokerURL, topicPrefix string) Config {
	return Config{
		BrokerURL:     brokerURL,
		ClientID:      "go-factory-io",
		TopicPrefix:   topicPrefix,
		QoS:           1,
		KeepAlive:     30 * time.Second,
		AutoReconnect: true,
		CleanSession:  true,
	}
}

// EventPayload matches the REST API event format for consistency.
type EventPayload struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Bridge publishes GEM events to an MQTT broker.
type Bridge struct {
	config Config
	logger *slog.Logger
	client paho.Client
	mu     sync.RWMutex
}

// NewBridge creates an MQTT bridge.
func NewBridge(cfg Config, logger *slog.Logger) *Bridge {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.KeepAlive == 0 {
		cfg.KeepAlive = 30 * time.Second
	}
	return &Bridge{
		config: cfg,
		logger: logger,
	}
}

// Connect establishes the MQTT broker connection.
func (b *Bridge) Connect() error {
	opts := paho.NewClientOptions()
	opts.AddBroker(b.config.BrokerURL)
	opts.SetClientID(b.config.ClientID)
	opts.SetKeepAlive(b.config.KeepAlive)
	opts.SetAutoReconnect(b.config.AutoReconnect)
	opts.SetCleanSession(b.config.CleanSession)

	if b.config.Username != "" {
		opts.SetUsername(b.config.Username)
		opts.SetPassword(b.config.Password)
	}
	if b.config.TLSConfig != nil {
		opts.SetTLSConfig(b.config.TLSConfig)
	}

	opts.SetOnConnectHandler(func(_ paho.Client) {
		b.logger.Info("MQTT connected", "broker", b.config.BrokerURL)
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		b.logger.Warn("MQTT connection lost", "error", err)
	})

	b.mu.Lock()
	b.client = paho.NewClient(opts)
	b.mu.Unlock()

	token := b.client.Connect()
	token.Wait()
	return token.Error()
}

// Close disconnects from the broker.
func (b *Bridge) Close() {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.client != nil && b.client.IsConnected() {
		b.client.Disconnect(1000)
		b.logger.Info("MQTT disconnected")
	}
}

// AttachToHandler registers GEM event callbacks that auto-publish to MQTT.
func (b *Bridge) AttachToHandler(h *gem.Handler) {
	h.OnEventSent(func(dataID, ceid uint32) {
		b.publish(fmt.Sprintf("%s/event/%d", b.config.TopicPrefix, ceid), EventPayload{
			Type:      "collection_event",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Data:      map[string]interface{}{"dataID": dataID, "ceid": ceid},
		})
	})

	h.OnAlarmSent(func(alid uint32, set bool, alarm *gem.Alarm) {
		state := "cleared"
		if set {
			state = "set"
		}
		b.publish(fmt.Sprintf("%s/alarm/%d", b.config.TopicPrefix, alid), EventPayload{
			Type:      "alarm",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Data: map[string]interface{}{
				"alid":  alid,
				"state": state,
				"text":  alarm.Text,
				"name":  alarm.Name,
			},
		})
	})
}

// PublishStatus publishes equipment status to {prefix}/status.
func (b *Bridge) PublishStatus(data interface{}) error {
	return b.publish(b.config.TopicPrefix+"/status", EventPayload{
		Type:      "status",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	})
}

// PublishSV publishes a status variable value to {prefix}/sv/{svid}.
func (b *Bridge) PublishSV(svid uint32, value interface{}) error {
	return b.publish(fmt.Sprintf("%s/sv/%d", b.config.TopicPrefix, svid), EventPayload{
		Type:      "status_variable",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      map[string]interface{}{"svid": svid, "value": value},
	})
}

func (b *Bridge) publish(topic string, payload EventPayload) error {
	b.mu.RLock()
	c := b.client
	b.mu.RUnlock()

	if c == nil || !c.IsConnected() {
		return fmt.Errorf("mqtt: not connected")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mqtt: marshal payload: %w", err)
	}

	token := c.Publish(topic, b.config.QoS, false, data)
	token.Wait()
	if err := token.Error(); err != nil {
		b.logger.Error("MQTT publish failed", "topic", topic, "error", err)
		return err
	}

	b.logger.Debug("MQTT published", "topic", topic)
	return nil
}
