package mqpool

import (
	"errors"

	mqerrors "github.com/20vikash/mqpool/internal/errors"
)

func (p *Pool) Validate() error {
	/*
		Validate() will validate all the fields in the Pool struct and throws an error if the config
		is not set properly or set wrongly.
		This function would serve as the middleware for all Pool based functions.

		1. If pooling mode is auto, p.AutoConfig.MinChannels should not be less than or equal to 0.
		2. If pooling mode is auto, p.AutoConfig should be set and initialized.
		3. If MaxChannels is not set to auto in p.AutoConfig, it should not be less than p.AutoConfig.MinChannels.
		4. If pooling mode is not auto, p.NChan should not be less than or equal to 0
	*/

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
