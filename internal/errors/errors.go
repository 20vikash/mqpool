package mqerrors

const (
	INVALID_QUEUE_TYPE  = "invalid queue type. Must be consts.PUBLISHER or consts.CONSUMER"
	INVALID_MIN_CHANNEL = "min channels must be > 0 in auto mode"
	INVALID_MAX_CHANNEL = "max channels must be >= min channels"
	INVALID_NCHAN       = "nChan must be > 0 in static mode"
)
