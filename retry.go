package mqpool

import amqp "github.com/rabbitmq/amqp091-go"

// RetryInit() initializes retry queue for r.MainQueue. It will create a new retry queue if
// r.Auto = true, otherwise uses r.RetryQueueName
func (r *RetryConfig) RetryInit() {
}

// This function will only be called if RetryConfig.Auto = true
func (r *RetryConfig) createRetryQueue(ch *amqp.Channel, queueName string, dlq string, pool *Pool) (*amqp.Queue, error) {
	q, err := ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		amqp.Table{
			"x-message-ttl":             int32(1000), // Wait 5 seconds
			"x-dead-letter-exchange":    "",          // Use default exchange
			"x-dead-letter-routing-key": dlq,         // Send back to main
		},
	)
	if err != nil {
		// Release the channel
		pool.PushChannel(ch)
		return nil, err
	}

	// Release the channel
	pool.PushChannel(ch)

	return &q, nil
}
