package mqpool

import (
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Mq struct {
	User         string
	Pass         string
	Port         string
	Host         string
	RetryCounter int
}

func (mq *Mq) ConnectToMq() (*amqp.Connection, error) {
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
