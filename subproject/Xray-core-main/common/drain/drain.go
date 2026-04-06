package drain

import "io"

type Drainer interface {
	AcknowledgeReceive(size int)
	Drain(reader io.Reader) error
}
