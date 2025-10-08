package mqpool

import (
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Mq struct {
	User         string // Username
	Pass         string // Password
	Port         string // Port number
	Host         string // Host name
	RetryCounter int    // Retry counter for connection
}

func (mq *Mq) ConnectToMq() (*amqp.Connection, error) {
	/*
		ConnectToMq() tries to connect to RabbitMQ server with the given MQ struct.
		Retries for MQ.Retrycounter times.
	*/

	var err error
	var conn *amqp.Connection
	for i := range mq.RetryCounter { // Retry for mq.RetryCounter times
		log.Printf("Attempt mq connection: %d", i+1)
		conn, err = amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%s/", mq.User, mq.Pass, mq.Host, mq.Port))
		if err != nil {
			time.Sleep(1 * time.Second)
		} else {
			return conn, nil
		}
	}

	return nil, err
}
