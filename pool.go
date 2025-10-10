package mqpool

import (
	"errors"

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

	if !p.Auto { // Static pooling
		chPool := &channelPool{
			Pool: make(chan *amqp.Channel, p.NChan),
			Conn: p.Conn,
		}

		for range p.NChan {
			ch, err := p.Conn.Channel()
			if err != nil {
				return nil, errors.New(mqerrors.CANNOT_CREATE_CHANNEL)
			}

			chPool.Pool <- ch
		}

		return chPool, nil
	}

	return nil, nil
}

// Stubs
func (p *Pool) PushChannel(ch *amqp.Channel) {

}

func (p *Pool) GetFreeChannel() *amqp.Channel {
	return nil
}
