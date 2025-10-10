package mqpool

import (
	"errors"

	mqerrors "github.com/20vikash/mqpool/internal/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RetryInit() initializes retry queue for r.MainQueue. It will create a new queue
// and retry queue using r.MainQueue and r.RetryQueue
// This function takes p as the Pool object,
// mainQueueConfig and retryQueueConfig as QueueConfig object that will be used to create queues.
// This function returns the main queue which has the retry queue attached.
func (r *RetryConfig) RetryInit(p *Pool, mainQueueConfig, retryQueueConfig *QueueConfig) (*amqp.Queue, error) {
	// Create a queue
	ch := p.GetFreeChannel()
	mainQueue, err := r.createQueueInstance(ch, r.MainQueue, r.RetryQueue, r.MainExchange, mainQueueConfig, p)
	if err != nil {
		return nil, errors.New(mqerrors.CANNOT_CREATE_MAIN_QUEUE)
	}

	ch = p.GetFreeChannel()

	_, err = r.createRetryQueue(ch, r.MainQueue, r.RetryQueue, r.RetryExchange, retryQueueConfig, p)
	if err != nil {
		return nil, errors.New(mqerrors.CANNOT_CREATE_RETRY_QUEUE)
	}

	return mainQueue, nil
}

// This function will only be called if RetryConfig.Auto = true
func (r *RetryConfig) createRetryQueue(ch *amqp.Channel, queueName string, dlq string, exchange string, queueConfig *QueueConfig, pool *Pool) (*amqp.Queue, error) {
	q, err := ch.QueueDeclare(
		queueName,                    // name
		queueConfig.Durable,          // durable
		queueConfig.DeleteWhenUnused, // delete when unused
		queueConfig.Exclusive,        // exclusive
		queueConfig.NoWait,           // no-wait
		amqp.Table{
			"x-message-ttl":             int32(r.TTL), // Wait r.TTL seconds
			"x-dead-letter-exchange":    exchange,     // Use user given exchange
			"x-dead-letter-routing-key": dlq,          // Send back to main
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

// This function creates a queue and attaches it with the retry Queue.
func (r *RetryConfig) createQueueInstance(ch *amqp.Channel, queueName string, retryQueue string, exchange string, queueConfig *QueueConfig, pool *Pool) (*amqp.Queue, error) {
	q, err := ch.QueueDeclare(
		queueName,                    // name
		queueConfig.Durable,          // durable
		queueConfig.DeleteWhenUnused, // delete when unused
		queueConfig.Exclusive,        // exclusive
		queueConfig.NoWait,           // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    exchange,   // Use default exchange
			"x-dead-letter-routing-key": retryQueue, // On failure, go here
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
