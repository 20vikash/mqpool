package mqpool

import amqp "github.com/rabbitmq/amqp091-go"

type RetryConfig struct {
	MainQueue      *amqp.Queue // Set this value to the main queue that the retry queue gets attached to
	Auto           bool        // Set it to true if retry queue wants to be handled automatically
	RetryQueueName string      // Set this field if Auto = false
	MaxRetries     int         // Max retries before NACK
	TTL            int         // Time to live before next attempt
}
