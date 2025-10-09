package mqerrors

const (
	INVALID_QUEUE_TYPE    = "invalid queue type. Must be consts.PUBLISHER or consts.CONSUMER"
	INVALID_MIN_CHANNEL   = "min channels must be > 0 in auto mode"
	INVALID_MAX_CHANNEL   = "max channels must be >= min channels"
	INVALID_NCHAN         = "nChan must be > 0 in static mode"
	MISSING_AUTO_CONFIG   = "autoConfig cannot be nil when Auto mode is enabled"
	CANNOT_CREATE_CHANNEL = "failed to create channel while initializing pool"
)
