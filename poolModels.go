package mqpool

import (
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Auto = -1
)

// AutoPool struct defines the Min and Max channels for dynamic channel resizing
type AutoPool struct {
	MinChannels        int32      // Minimum number of channels(idle)
	MaxChannels        int32      // Max number of channels
	waitTime           chan int64 // Wait time of a specific borrow in milliseconds
	timeOut            chan bool  // will be true if a specific GetFreeChannel timesout
	acquireWaitTimeAvg int64      // Avg of all wait durations in milliseconds.
	acquireTimeouts    int32      // Count of GetFreeChannel() timeouts.
	size               int32      // Size of the pool
}

// Pool struct is the main struct to initialize channel pooling.
type Pool struct {
	Conn       *amqp.Connection // The MQ connection object
	Auto       bool             // Set it to true if dynamic pooling is required
	NChan      int32            // NChan is the number of channels in the pool for static pool(if Auto = false)
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
	Pool     chan *channelModel // Buffered chan of *amqp.Channel which is the actual pool
	Conn     *amqp.Connection   // The connection that handles all the channels
	auto     bool               // will be set to true if Pool.Auto = true
	autoPool *AutoPool          // will be automatically set if Pool.Auto = true
}

// channelModel represents the amqp channel and taken state.
type channelModel struct {
	ch       *amqp.Channel // amqp channel
	taken    bool          // true if its taken by a producer or consumer, false otherwise
	lastUsed time.Time
}
