package mqpool

const (
	Auto = -1
)

const (
	PUBLISHER = "publisher"
	CONSUMER  = "consumer"
)

type AutoPool struct {
	MinChannels int // Minimum number of channels(idle)
	MaxChannels int // Max number of channels Set it to Auto for calculating this number automatically.
}

type PoolConfig struct {
	Type     string // Set it to either PUBLISHER or CONSUMER
	Auto     bool   // Set it to true if dynamic pooling is required
	NChan    int    // NChan is the number of channels in the pool for static pool(if Auto = false)
	AutoPool        // Initialize this if Auto = true
}
