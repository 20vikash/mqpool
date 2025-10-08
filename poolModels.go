package mqpool

const (
	Auto = -1
)

/*
AutoPool struct defines the Min and Max channels for dynamic channel resizing
*/
type AutoPool struct {
	MinChannels int // Minimum number of channels(idle)
	MaxChannels int // Max number of channels Set it to Auto for calculating this number automatically.
}

/*
Pool struct is the main struct to initialize channel pooling.
*/
type Pool struct {
	Auto       bool      // Set it to true if dynamic pooling is required
	NChan      int       // NChan is the number of channels in the pool for static pool(if Auto = false)
	AutoConfig *AutoPool // Initialize this if Auto = true
}

/*
RetryConfig struct is optional if queues wants to get binded with retry queues.
*/
type RetryConfig struct {
	MainQueue      string // Set this value to the main queue that the retry queue gets attached to
	Auto           bool   // Set it to true if retry queue wants to be handled automatically
	RetryQueueName string // Set this field if Auto = false
	MaxRetries     int    // Max retries before NACK
	TTL            int    // Time to live before next attempt
}
