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
		Pool: make(chan *channelModel, num),
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

		chPool.Pool <- &channelModel{ch: ch, taken: false}
	}

	// Length of the pool
	chPool.autoPool.size = len(chPool.Pool)

	if p.Auto {
		// Spin up pool listener to auto scale the pool
		go p.autoPoolListen(chPool)
		chPool.kill() // goroutine listens in the background and closes idle channels
	}

	return chPool, nil
}

// PushChannel() will push ch into p.Pool
//
// Reminder: Call this once you are done with an atomic mq operation to release the channel
func (p *channelPool) PushChannel(ch *amqp.Channel) error {
	chModel := &channelModel{ch: ch, taken: false, lastUsed: time.Now()}

	if chModel.ch.IsClosed() { // Makes sure closed channel dont get pushed
		channel, err := p.Conn.Channel()
		if err != nil {
			return err
		}
		chModel = &channelModel{ch: channel, taken: false, lastUsed: time.Now()}
	}

	select {
	case p.Pool <- chModel:
	default:
		err := ch.Close() // pool full, close the extra channel
		if err != nil {
			return err
		}
	}

	return nil
}

// kill() will close the channel after it exceedes its idle timeout.
//
// If the channel is idle for 1 minute, close it(scale down)
func (p *channelPool) kill() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
		L:
			for {
				select {
				case ch := <-p.Pool:
					if !ch.taken && time.Since(ch.lastUsed) > 1*time.Minute && p.autoPool.size > p.autoPool.MinChannels {
						ch.ch.Close()

						p.lock.Lock()
						p.autoPool.size--
						p.lock.Unlock()
					} else {
						p.Pool <- ch
					}
				default:
					break L
				}
			}
		}
	}()
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
		if ch.ch.IsClosed() { // Close in other part of code, or broker closed it
			newCh, err := p.Conn.Channel()
			if err != nil {
				return nil, err
			}
			if prefetchCounter > 0 {
				newCh.Qos(prefetchCounter, 0, false)
			}

			if p.auto {
				elapsed := time.Since(t)
				p.autoPool.waitTime <- elapsed.Milliseconds()
			}

			return newCh, nil
		}

		if p.auto {
			elapsed := time.Since(t)
			p.autoPool.waitTime <- elapsed.Milliseconds()
		}

		ch.taken = true

		return ch.ch, nil
	case <-ctx.Done():
		if p.auto {
			p.autoPool.timeOut <- true
		}
		return nil, ctx.Err()
	}
}

// autoPoolListen() will listen for metrics in the background for auto-scaling the pool
func (p *Pool) autoPoolListen(ch *channelPool) {
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
			p.evaluateScale(ch) // perform auto-scaling logic every 2s
		}
	}
}

// evaluateScale() scales the pool based on conditions
func (p *Pool) evaluateScale(ch *channelPool) {
	currentSizeWithoutClosed := ch.autoPool.size
	currentSizeWithClosed := len(ch.Pool)

	for range max(currentSizeWithClosed-currentSizeWithoutClosed, 0) {
		c := <-ch.Pool
		c.ch.Close()
	}

	avgWait := p.AutoConfig.acquireWaitTimeAvg
	timeouts := p.AutoConfig.acquireTimeouts

	max := p.AutoConfig.MaxChannels

	// Scale Up (avg wait time is greater than 100ms or there are 2+ timeouts)
	if (avgWait > 100 || timeouts > 2) && currentSizeWithoutClosed < max {
		step := 1
		if avgWait > 200 { // Upscale the step if avg wait time is more than 200ms
			step = 2
		}

		newSize := min(currentSizeWithoutClosed+step, max)

		p.resizePool(newSize, ch)
		p.AutoConfig.acquireTimeouts = 0 // reset counter
		return
	}
}

// resizePool() will scale up by step
func (p *Pool) resizePool(newSize int, ch *channelPool) error {
	step := newSize - len(ch.Pool) // Guarenteed its always scale up

	for range step {
		channel, err := p.Conn.Channel()
		if err != nil {
			return err
		}

		ch.lock.Lock()
		ch.autoPool.size++
		ch.lock.Unlock()

		ch.Pool <- &channelModel{ch: channel, taken: false, lastUsed: time.Now()}
	}

	return nil
}
