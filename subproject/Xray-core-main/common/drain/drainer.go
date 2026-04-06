package drain

import (
	"io"

	"github.com/xtls/xray-core/common/dice"
	"github.com/xtls/xray-core/common/errors"
)

type BehaviorSeedLimitedDrainer struct {
	DrainSize int
}

func NewBehaviorSeedLimitedDrainer(behaviorSeed int64, drainFoundation, maxBaseDrainSize, maxRandDrain int) (Drainer, error) {
	behaviorRand := dice.NewDeterministicDice(behaviorSeed)
	BaseDrainSize := behaviorRand.Roll(maxBaseDrainSize)
	RandDrainMax := behaviorRand.Roll(maxRandDrain) + 1
	RandDrainRolled := dice.Roll(RandDrainMax)
	DrainSize := drainFoundation + BaseDrainSize + RandDrainRolled
	return &BehaviorSeedLimitedDrainer{DrainSize: DrainSize}, nil
}

func (d *BehaviorSeedLimitedDrainer) AcknowledgeReceive(size int) {
	d.DrainSize -= size
}

func (d *BehaviorSeedLimitedDrainer) Drain(reader io.Reader) error {
	if d.DrainSize > 0 {
		err := drainReadN(reader, d.DrainSize)
		if err == nil {
			return errors.New("drained connection")
		}
		return errors.New("unable to drain connection").Base(err)
	}
	return nil
}

func drainReadN(reader io.Reader, n int) error {
	_, err := io.CopyN(io.Discard, reader, int64(n))
	return err
}

func WithError(drainer Drainer, reader io.Reader, err error) error {
	drainErr := drainer.Drain(reader)
	if drainErr == nil {
		return err
	}
	return errors.New(drainErr).Base(err)
}

type NopDrainer struct{}

func (n NopDrainer) AcknowledgeReceive(size int) {
}

func (n NopDrainer) Drain(reader io.Reader) error {
	return nil
}

func NewNopDrainer() Drainer {
	return &NopDrainer{}
}
