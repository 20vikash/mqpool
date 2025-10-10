package mqpool

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Auto = -1
)

// AutoPool struct defines the Min and Max channels for dynamic channel resizing
type AutoPool struct {
	MinChannels int // Minimum number of channels(idle)
	MaxChannels int // Max number of channels Set it to Auto for calculating this number automatically.
}

// Pool struct is the main struct to initialize channel pooling.
type Pool struct {
	Conn       *amqp.Connection // The MQ connection object
	Auto       bool             // Set it to true if dynamic pooling is required
	NChan      int              // NChan is the number of channels in the pool for static pool(if Auto = false)
	AutoConfig *AutoPool        // Initialize this if Auto = true
}

// RetryConfig struct is optional if queues wants to get binded with retry queues.
type RetryConfig struct {
	MainQueue     string // Set this value to the main queue that the retry queue gets attached to
	RetryQueue    string // Set this field if Auto = false
	MainExchange  string // Exchange name for the main queue
	RetryExchange string // Exchange for the retry queue
	MaxRetries    int    // Max retries before NACK
	TTL           int    // Time to live before next attempt
}

// QueueConfig contains the fields that is required to create a queue
type QueueConfig struct {
	Durable          bool
	DeleteWhenUnused bool
	Exclusive        bool
	NoWait           bool
}

// channelPool struct is what you get once you initialize pool using Pool.Init()
type channelPool struct {
	Pool chan *amqp.Channel // Buffered chan of *amqp.Channel which is the actual pool
	Conn *amqp.Connection   // The connection that handles all the channels
}
