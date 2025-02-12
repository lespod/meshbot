package meshwrapper

import (
	"fmt"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
)

type Channel struct {
	id      uint32
	name    string
	passkey []byte
}

func NewChannel(unit *meshtastic.Channel) Channel {
	if unit == nil {
		return Channel{}
	}
	return Channel{
		id:      uint32(unit.Index),
		name:    unit.GetSettings().Name,
		passkey: unit.GetSettings().Psk,
	}
}

func (c Channel) String() string {
	return fmt.Sprintf("[%d] %s", c.id, c.name)
}
