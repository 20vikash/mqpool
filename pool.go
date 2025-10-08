package mqpool

import (
	"errors"

	mqerrors "github.com/20vikash/mqpool/internal/errors"
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
func (p *Pool) Validate() error {
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
