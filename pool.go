package mqpool

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
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

	var num int32

	if !p.Auto { // Static pooling
		num = p.NChan
	} else {
		num = p.AutoConfig.MaxChannels
	}
	chPool := &channelPool{
		Pool:   make(chan *channelModel, num),
		Conn:   p.Conn,
		auto:   p.Auto,
		closed: make(chan struct{}),
	}

	if p.Auto {
		// Initialize AutoPool fields if p.Auto = true
		chPool.autoPool = p.AutoConfig
		chPool.autoPool.waitTime = make(chan int64, p.AutoConfig.MaxChannels)
		chPool.autoPool.timeOut = make(chan bool)
		chPool.autoPool.acquireWaitTimeAvg = 0
		chPool.autoPool.acquireTimeouts = 0

		// Set num to minChannels
		num = p.AutoConfig.MinChannels
	}

	for range num {
		ch, err := p.Conn.Channel()
		if err != nil {
			return nil, errors.New(mqerrors.CANNOT_CREATE_CHANNEL)
		}
		chModel := &channelModel{ch: ch, taken: atomic.Bool{}}
		chModel.taken.Store(false)

		chPool.Pool <- chModel
	}

	if p.Auto {
		// Length of the pool
		chPool.autoPool.size = int32(num)

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
	chModel := &channelModel{ch: ch, taken: atomic.Bool{}, lastUsed: time.Now()}
	chModel.taken.Store(false)

	if chModel.ch.IsClosed() { // Makes sure closed channel dont get pushed
		channel, err := p.Conn.Channel()
		if err != nil {
			return err
		}
		chModel = &channelModel{ch: channel, taken: atomic.Bool{}, lastUsed: time.Now()}
		chModel.taken.Store(false)
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

		for {
			select {
			case <-p.closed:
				return
			case <-ticker.C:
			L:
				for {
					select {
					case ch := <-p.Pool:
						if !ch.taken.Load() && time.Since(ch.lastUsed) > 30*time.Second && p.autoPool.size > p.autoPool.MinChannels {
							ch.ch.Close()

							atomic.AddInt32(&p.autoPool.size, -1)
						} else {
							select {
							case p.Pool <- ch:
							default:
							}
						}
					default:
						break L
					}
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
				select {
				case p.autoPool.waitTime <- elapsed.Milliseconds():
				default:
				}
			}

			return newCh, nil
		}

		if p.auto {
			elapsed := time.Since(t)
			select {
			case p.autoPool.waitTime <- elapsed.Milliseconds():
			default:
			}
		}

		ch.taken.Store(true)

		return ch.ch, nil
	case <-ctx.Done():
		if p.auto {
			select {
			case p.autoPool.timeOut <- true:
			default:
			}
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
		case <-ch.closed:
			return // Gracefully shutdown
		case newWait := <-p.AutoConfig.waitTime:
			// Compute EMA for average wait time
			newAvg := alpha*float64(newWait) + (1-alpha)*float64(atomic.LoadInt64(&p.AutoConfig.acquireWaitTimeAvg))
			atomic.StoreInt64(&p.AutoConfig.acquireWaitTimeAvg, int64(newAvg))

		case <-p.AutoConfig.timeOut:
			atomic.AddInt32(&p.AutoConfig.acquireTimeouts, 1)

		case <-ticker.C:
			p.evaluateScale(ch) // perform auto-scaling logic every 2s
		}
	}
}

// evaluateScale() scales the pool based on conditions
func (p *Pool) evaluateScale(ch *channelPool) {
	activePoolChans := atomic.LoadInt32(&ch.autoPool.size)
	TotalPoolChans := int32(len(ch.Pool))

	diff := TotalPoolChans - activePoolChans
	if diff > 0 {
	L:
		for range diff {
			select {
			case c := <-ch.Pool:
				if !c.ch.IsClosed() {
					c.ch.Close()
				}
				atomic.AddInt32(&ch.autoPool.size, -1)
				activePoolChans = ch.autoPool.size
			default:
				break L
			}
		}
	}

	avgWait := atomic.LoadInt64(&p.AutoConfig.acquireWaitTimeAvg)
	timeouts := atomic.LoadInt32(&p.AutoConfig.acquireTimeouts)

	max := p.AutoConfig.MaxChannels

	// Scale Up (avg wait time is greater than 100ms or there are 2+ timeouts)
	if (avgWait > 100 || timeouts > 2) && activePoolChans < max {
		step := 1
		if avgWait > 200 { // Upscale the step if avg wait time is more than 200ms
			step = 2
		}
		if avgWait > 500 {
			step = 4
		}

		newSize := min(activePoolChans+int32(step), max)

		p.resizePool(int(newSize), ch)
		atomic.StoreInt32(&p.AutoConfig.acquireTimeouts, 0) // reset counter
		return
	}
}

// resizePool() will scale up by step
func (p *Pool) resizePool(newSize int, ch *channelPool) {
	step := int32(newSize) - atomic.LoadInt32(&ch.autoPool.size) // Guarenteed its always scale up

	for range step {
		channel, err := p.Conn.Channel()
		if err != nil {
			fmt.Println("Failed to create a new channel while resizing")
		} else {
			atomic.AddInt32(&ch.autoPool.size, 1)
			chModel := &channelModel{ch: channel, taken: atomic.Bool{}, lastUsed: time.Now()}
			chModel.taken.Store(false)

			ch.Pool <- chModel
		}
	}
}

// Gracefully closes the pool
func (p *channelPool) Close() {
	close(p.closed)
	for {
		select {
		case ch := <-p.Pool:
			ch.ch.Close()
		default:
			return
		}
	}
}
