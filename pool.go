package mqpool

import (
	"context"
	"errors"
	"time"

	mqerrors "github.com/20vikash/mqpool/internal/errors"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Validate checks all the fields in the Pool struct and returns an error
// if the configuration is invalid. This function acts as a middleware
// for all Pool-based operations.
//
// Validation rules:
//  1. If pooling mode is auto, p.AutoConfig must be set and initialized.
//  2. p.AutoConfig.MinChannels must be greater than 0 in auto mode.
//  3. If MaxChannels is not set to Auto in p.AutoConfig, it must be
//     greater than or equal to p.AutoConfig.MinChannels.
//  4. If pooling mode is not auto, p.NChan must be greater than 0.
func (p *Pool) validate() error { // Non exported method
	if p.Auto {
		if p.AutoConfig == nil {
			return errors.New(mqerrors.MISSING_AUTO_CONFIG)
		}
		if p.AutoConfig.MinChannels <= 0 {
			return errors.New(mqerrors.INVALID_MIN_CHANNEL)
		}
		if p.AutoConfig.MaxChannels != Auto && p.AutoConfig.MaxChannels < p.AutoConfig.MinChannels {
			return errors.New(mqerrors.INVALID_MAX_CHANNEL)
		}
	} else if p.NChan <= 0 {
		return errors.New(mqerrors.INVALID_NCHAN)
	}

	return nil
}

// Init() will Initialize the mq channel pool either statically or dynamically
// based on Pool.Auto
func (p *Pool) Init() (*channelPool, error) { // Exported method
	// Validate the Pool object before initializing pool
	err := p.validate()
	if err != nil {
		return nil, err
	}

	var num int

	if !p.Auto { // Static pooling
		num = p.NChan
	} else {
		num = p.AutoConfig.MinChannels
	}
	chPool := &channelPool{
		Pool: make(chan *amqp.Channel, num),
		Conn: p.Conn,
		auto: p.Auto,
	}

	if p.Auto {
		// Initialize AutoPool fields if p.Auto = true
		chPool.autoPool = p.AutoConfig
		chPool.autoPool.waitTime = make(chan int64, p.AutoConfig.MaxChannels)
		chPool.autoPool.timeOut = make(chan bool)
		chPool.autoPool.acquireWaitTimeAvg = 0
		chPool.autoPool.acquireTimeouts = 0

		// Set num to maxChannels
		num = p.AutoConfig.MaxChannels
	}

	for range num {
		ch, err := p.Conn.Channel()
		if err != nil {
			return nil, errors.New(mqerrors.CANNOT_CREATE_CHANNEL)
		}

		chPool.Pool <- ch
	}

	if p.Auto {
		// Spin up pool listener to auto scale the pool
		go p.autoPoolListen()
	}

	return chPool, nil
}

// PushChannel() will push ch into p.Pool
//
// Reminder: Call this once you are done with an atomic mq operation to release the channel
func (p *channelPool) PushChannel(ch *amqp.Channel) error {
	select {
	case p.Pool <- ch:
	default:
		err := ch.Close() // pool full, close the extra channel
		if err != nil {
			return err
		}
	}

	return nil
}

// GetFreeChannel() will try to get a free unused channel from the pool.
// If it fetches a closed channel, it will create a new channel at the moment
//
// PrefetchCounter ensures the number of messages to send to the consumer before acks.
//
// PrefetchCounter will be ignored if the channel is going to be used for Producing. Pass 0 if the channel is
// going to be used for producing
func (p *channelPool) GetFreeChannel(ctx context.Context, prefetchCounter int) (*amqp.Channel, error) {
	var t time.Time

	if p.auto {
		t = time.Now()
	}

	select {
	case ch := <-p.Pool:
		if ch.IsClosed() { // Close in other part of code, or broker closed it
			newCh, err := p.Conn.Channel()
			if err != nil {
				return nil, err
			}
			if prefetchCounter > 0 {
				newCh.Qos(prefetchCounter, 0, false)
			}
			return newCh, nil
		}

		if p.auto {
			elapsed := time.Since(t)
			p.autoPool.waitTime <- elapsed.Milliseconds()
		}
		return ch, nil
	case <-ctx.Done():
		if p.auto {
			p.autoPool.timeOut <- true
		}
		return nil, ctx.Err()
	}
}

// autoPoolListen() will listen for metrics in the background for auto-scaling the pool
func (p *Pool) autoPoolListen() {
	alpha := 0.3 // Balanced responsiveness
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case newWait := <-p.AutoConfig.waitTime:
			// Compute EMA for average wait time
			newAvg := alpha*float64(newWait) + (1-alpha)*float64(p.AutoConfig.acquireWaitTimeAvg)
			p.AutoConfig.acquireWaitTimeAvg = int64(newAvg)

		case <-p.AutoConfig.timeOut:
			p.AutoConfig.acquireTimeouts++

		case <-ticker.C:
			p.evaluateScale() // perform auto-scaling logic every 2s
		}
	}
}

// Stub
func (p *Pool) evaluateScale() {

}
