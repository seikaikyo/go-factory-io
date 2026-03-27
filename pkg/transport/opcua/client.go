// Package opcua provides an OPC-UA client transport for industrial equipment communication.
// Wraps the gopcua library with the go-factory-io transport interface pattern.
package opcua

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/monitor"
	"github.com/gopcua/opcua/ua"
)

// Config holds OPC-UA connection parameters.
type Config struct {
	// Endpoint is the OPC-UA server URL (e.g., "opc.tcp://192.168.1.100:4840").
	Endpoint string

	// SecurityPolicy: None, Basic128Rsa15, Basic256, Basic256Sha256.
	SecurityPolicy string

	// SecurityMode: None, Sign, SignAndEncrypt.
	SecurityMode string

	// Certificate and key paths for secure connections.
	CertFile string
	KeyFile  string

	// Auth: anonymous, username/password, or certificate.
	Username string
	Password string

	// RequestTimeout for read/write/browse operations.
	RequestTimeout time.Duration
}

// DefaultConfig returns an OPC-UA config with common defaults.
func DefaultConfig(endpoint string) Config {
	return Config{
		Endpoint:       endpoint,
		SecurityPolicy: "None",
		SecurityMode:   "None",
		RequestTimeout: 10 * time.Second,
	}
}

// Client wraps a gopcua OPC-UA client with convenience methods.
type Client struct {
	config Config
	logger *slog.Logger
	client *opcua.Client

	mu   sync.RWMutex
	subs map[uint32]*monitor.Subscription
}

// NewClient creates a new OPC-UA client.
func NewClient(config Config, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		config: config,
		logger: logger,
		subs:   make(map[uint32]*monitor.Subscription),
	}
}

// Connect establishes the OPC-UA connection.
func (c *Client) Connect(ctx context.Context) error {
	c.logger.Info("OPC-UA connecting", "endpoint", c.config.Endpoint)

	opts := []opcua.Option{
		opcua.SecurityPolicy(c.config.SecurityPolicy),
	}

	if c.config.SecurityMode != "" && c.config.SecurityMode != "None" {
		mode := ua.MessageSecurityModeFromString(c.config.SecurityMode)
		opts = append(opts, opcua.SecurityMode(mode))
	}

	if c.config.CertFile != "" && c.config.KeyFile != "" {
		opts = append(opts,
			opcua.CertificateFile(c.config.CertFile),
			opcua.PrivateKeyFile(c.config.KeyFile),
		)
	}

	if c.config.Username != "" {
		opts = append(opts, opcua.AuthUsername(c.config.Username, c.config.Password))
	} else {
		opts = append(opts, opcua.AuthAnonymous())
	}

	client, err := opcua.NewClient(c.config.Endpoint, opts...)
	if err != nil {
		return fmt.Errorf("opcua: create client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("opcua: connect %s: %w", c.config.Endpoint, err)
	}

	c.mu.Lock()
	c.client = client
	c.mu.Unlock()

	c.logger.Info("OPC-UA connected", "endpoint", c.config.Endpoint)
	return nil
}

// Close disconnects from the OPC-UA server.
func (c *Client) Close() error {
	c.mu.Lock()
	client := c.client
	c.client = nil
	c.mu.Unlock()

	if client != nil {
		return client.Close(context.Background())
	}
	return nil
}

// --- Read Operations ---

// ReadValue reads a single node value by NodeID string (e.g., "ns=2;s=Temperature").
func (c *Client) ReadValue(ctx context.Context, nodeID string) (*ua.Variant, error) {
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("opcua: parse node ID %q: %w", nodeID, err)
	}

	req := &ua.ReadRequest{
		MaxAge:             0,
		NodesToRead:        []*ua.ReadValueID{{NodeID: id, AttributeID: ua.AttributeIDValue}},
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("opcua: not connected")
	}

	resp, err := client.Read(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("opcua: read %s: %w", nodeID, err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("opcua: no results for %s", nodeID)
	}

	result := resp.Results[0]
	if result.Status != ua.StatusOK {
		return nil, fmt.Errorf("opcua: read %s status: %s", nodeID, result.Status)
	}

	return result.Value, nil
}

// ReadMultiple reads multiple node values at once.
func (c *Client) ReadMultiple(ctx context.Context, nodeIDs []string) (map[string]*ua.Variant, error) {
	nodesToRead := make([]*ua.ReadValueID, len(nodeIDs))
	for i, nid := range nodeIDs {
		parsed, err := ua.ParseNodeID(nid)
		if err != nil {
			return nil, fmt.Errorf("opcua: parse node ID %q: %w", nid, err)
		}
		nodesToRead[i] = &ua.ReadValueID{NodeID: parsed, AttributeID: ua.AttributeIDValue}
	}

	req := &ua.ReadRequest{
		MaxAge:             0,
		NodesToRead:        nodesToRead,
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("opcua: not connected")
	}

	resp, err := client.Read(ctx, req)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*ua.Variant, len(nodeIDs))
	for i, r := range resp.Results {
		if r.Status == ua.StatusOK {
			result[nodeIDs[i]] = r.Value
		}
	}
	return result, nil
}

// --- Write Operations ---

// WriteValue writes a value to a node.
func (c *Client) WriteValue(ctx context.Context, nodeID string, value *ua.Variant) error {
	nid, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("opcua: parse node ID: %w", err)
	}

	req := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      nid,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					Value: value,
				},
			},
		},
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("opcua: not connected")
	}

	resp, err := client.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("opcua: write: %w", err)
	}

	if len(resp.Results) > 0 && resp.Results[0] != ua.StatusOK {
		return fmt.Errorf("opcua: write status: %s", resp.Results[0])
	}
	return nil
}

// --- Browse Operations ---

// BrowseNode returns child nodes of a given node.
func (c *Client) BrowseNode(ctx context.Context, nodeID string) ([]*NodeInfo, error) {
	nid, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("opcua: parse node ID: %w", err)
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("opcua: not connected")
	}

	browseReq := &ua.BrowseRequest{
		NodesToBrowse: []*ua.BrowseDescription{
			{
				NodeID:          nid,
				BrowseDirection: ua.BrowseDirectionForward,
				ReferenceTypeID: ua.NewNumericNodeID(0, id.HierarchicalReferences),
				IncludeSubtypes: true,
				ResultMask:      uint32(ua.BrowseResultMaskAll),
			},
		},
	}

	resp, err := client.Browse(ctx, browseReq)
	if err != nil {
		return nil, err
	}

	if len(resp.Results) == 0 {
		return nil, nil
	}

	var nodes []*NodeInfo
	for _, ref := range resp.Results[0].References {
		nodes = append(nodes, &NodeInfo{
			NodeID:      ref.NodeID.NodeID.String(),
			BrowseName:  ref.BrowseName.Name,
			DisplayName: ref.DisplayName.Text,
			NodeClass:   ref.NodeClass.String(),
		})
	}
	return nodes, nil
}

// NodeInfo holds metadata about an OPC-UA node.
type NodeInfo struct {
	NodeID      string
	BrowseName  string
	DisplayName string
	NodeClass   string
}

// --- Subscription ---

// SubscriptionHandler processes data change notifications.
type SubscriptionHandler func(nodeID string, value *ua.Variant, timestamp time.Time)

// Subscribe creates a monitored subscription for the given nodes.
// The handler is called whenever a value changes.
func (c *Client) Subscribe(ctx context.Context, nodeIDs []string, interval time.Duration, handler SubscriptionHandler) (uint32, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()
	if client == nil {
		return 0, fmt.Errorf("opcua: not connected")
	}

	m, err := monitor.NewNodeMonitor(client)
	if err != nil {
		return 0, fmt.Errorf("opcua: create monitor: %w", err)
	}

	ch := make(chan *monitor.DataChangeMessage, 256)
	sub, err := m.ChanSubscribe(
		ctx,
		&opcua.SubscriptionParameters{Interval: interval},
		ch,
		nodeIDs...,
	)
	if err != nil {
		return 0, fmt.Errorf("opcua: subscribe: %w", err)
	}

	c.mu.Lock()
	c.subs[sub.SubscriptionID()] = sub
	c.mu.Unlock()

	// Process data changes
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if msg.Error != nil {
					c.logger.Error("OPC-UA subscription error", "error", msg.Error)
					continue
				}
				handler(msg.NodeID.String(), msg.Value, msg.SourceTimestamp)
			}
		}
	}()

	c.logger.Info("OPC-UA subscribed",
		"nodes", len(nodeIDs),
		"interval", interval,
		"subscriptionID", sub.SubscriptionID(),
	)

	return sub.SubscriptionID(), nil
}

// Unsubscribe removes a subscription.
func (c *Client) Unsubscribe(ctx context.Context, subID uint32) error {
	c.mu.Lock()
	sub, ok := c.subs[subID]
	if ok {
		delete(c.subs, subID)
	}
	c.mu.Unlock()

	if !ok {
		return fmt.Errorf("opcua: subscription %d not found", subID)
	}

	return sub.Unsubscribe(ctx)
}
