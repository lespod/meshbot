package meshwrapper

import (
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"github.com/timendus/meshbot/config"
	"github.com/timendus/meshbot/meshbot"
	"github.com/timendus/meshbot/meshwrapper/helpers"
)

const (
	MESSAGE_TYPE_TEXT_MESSAGE          = "text message"
	MESSAGE_TYPE_NODE_INFO             = "node info"
	MESSAGE_TYPE_POSITION              = "position"
	MESSAGE_TYPE_NEIGHBOR_INFO         = "neighbor info"
	MESSAGE_TYPE_ROUTING               = "routing"
	MESSAGE_TYPE_TRACEROUTE            = "traceroute"
	MESSAGE_TYPE_TELEMETRY_DEVICE      = "device telemetry"
	MESSAGE_TYPE_TELEMETRY_ENVIRONMENT = "environment telemetry"
	MESSAGE_TYPE_TELEMETRY_HEALTH      = "health telemetry"
	MESSAGE_TYPE_TELEMETRY_AIR_QUALITY = "air quality telemetry"
	MESSAGE_TYPE_TELEMETRY_POWER       = "power telemetry"
	MESSAGE_TYPE_TELEMETRY_LOCAL_STATS = "local stats telemetry"
	MESSAGE_TYPE_OTHER                 = "other"

	DEFAULT_BLOCKING_MESSAGE_TIMEOUT = 60 * time.Second
)

type Message struct {
	FromNode      *Node
	ToNode        *Node
	ReceivingNode *ConnectedNode

	Timestamp time.Time
	Snr       float32
	HopsAway  uint32

	MessageType        string
	Text               string
	DeviceMetrics      *meshtastic.DeviceMetrics
	EnvironmentMetrics *meshtastic.EnvironmentMetrics
	HealthMetrics      *meshtastic.HealthMetrics
	AirQualityMetrics  *meshtastic.AirQualityMetrics
	PowerMetrics       *meshtastic.PowerMetrics
	LocalStats         *meshtastic.LocalStats
	NeighborInfo       *meshtastic.NeighborInfo
	Position           *position
}

func (m Message) Reply(message string, timeout ...time.Duration) chan bool {
	ch := make(chan bool)

	go func() {
		messageTimeout := DEFAULT_BLOCKING_MESSAGE_TIMEOUT
		if len(timeout) > 0 {
			messageTimeout = timeout[0]
		}

		for _, msg := range helpers.BreakMessage(message) {
			ok := <-m.send(msg, messageTimeout)
			if !ok {
				ch <- false
				return
			}
		}

		ch <- true
	}()

	return ch
}

func (m *Message) send(message string, timeout time.Duration) chan bool {
	ch := make(chan bool)
	id := m.sendTextMessage(message)
	m.ReceivingNode.Acks[id] = ch
	go func() {
		time.Sleep(timeout)
		ch <- false
		delete(m.ReceivingNode.Acks, id)
	}()
	return ch
}

func (m *Message) sendTextMessage(message string) uint32 {
	helpers.Assert(m.ReceivingNode != nil, "Can't send a message without knowing through which device to send it")
	helpers.Assert(m.FromNode != nil, "Can't send a message to an unknown node")
	helpers.Assert(m.ToNode != nil, "Can't send a message from an unknown node")

	id := rand.Uint32()
	cfg := config.GetConfig()
	nodeAllowed := cfg.Settings.TransmitExceptionNodeId != 0 && m.FromNode.Id == cfg.Settings.TransmitExceptionNodeId
	if !cfg.Settings.AllowTransmit && !nodeAllowed {
		log.Println("WARNING: Transmission is not allowed by configuration. Attempted to send: " + message)
		return id
	}

	// Show we're transmitting this on the console. TODO: move this out of this
	// package. Same with error log above; this code should not be logging
	// stuff.
	log.Println(m.toReplyString(message))

	m.ReceivingNode.SendMessage(meshtastic.ToRadio_Packet{
		Packet: &meshtastic.MeshPacket{
			Id:       id,
			To:       m.FromNode.Id,
			From:     m.ToNode.Id,
			HopLimit: min(m.HopsAway+2, 7),
			WantAck:  true,
			Priority: meshtastic.MeshPacket_Priority(meshtastic.MeshPacket_Priority_value["RELIABLE"]),
			PayloadVariant: &meshtastic.MeshPacket_Decoded{
				Decoded: &meshtastic.Data{
					Portnum: meshtastic.PortNum_TEXT_MESSAGE_APP,
					Payload: []byte(message),
				},
			},
		},
	})
	return id
}

// Implement meshbot.ChatMessage interface

func (m Message) GetText() string {
	return m.Text
}

func (m Message) IsPrivateMessage() bool {
	return m.ToNode != nil && m.ToNode.Id != Broadcast.Id
}

func (m Message) GetType() string {
	return m.MessageType
}

func (m Message) GetChannelName() string {
	panic("TODO: implement")
	// return ""
}

func (m Message) GetSenderNode() meshbot.ChatUser {
	return m.FromNode
}

func (m Message) GetReceiverNode() meshbot.ChatUser {
	return m.ToNode
}

func (m Message) FindNode(needle string) meshbot.ChatUser {
	if m.ReceivingNode == nil {
		return nil
	}
	return m.ReceivingNode.NodeList.findNode(needle)
}

func (m Message) String() string {
	direction := ""
	if m.FromNode != nil {
		direction += m.FromNode.String()
	} else {
		direction += "No node"
	}
	if m.ToNode != nil {
		direction += " -> " + m.ToNode.String()
	} else {
		direction += " -> No node"
	}

	if m.MessageType == MESSAGE_TYPE_NEIGHBOR_INFO {
		neighbours := "unknown"
		if m.FromNode != nil {
			neighbours = m.FromNode.Neighbors.String()
		}
		return fmt.Sprintf("%s: \033[1mNeighbor list:\033[0m %s %s", direction, m.radioMetricsString(), neighbours)
	}

	var content string
	if m.MessageType == MESSAGE_TYPE_TEXT_MESSAGE {
		content = m.Text
	} else {
		content = "\033[1m" + m.MessageType + " packet\033[0m"
	}

	return fmt.Sprintf("%s: %s %s", direction, content, m.radioMetricsString())
}

func (m *Message) toReplyString(message string) string {
	direction := ""
	if m.ToNode != nil {
		direction += m.ToNode.String()
	} else {
		direction += "No node"
	}
	if m.FromNode != nil {
		direction += " -> " + m.FromNode.String()
	} else {
		direction += " -> No node"
	}

	return fmt.Sprintf("%s: %s", direction, message)
}

func (m *Message) radioMetricsString() string {
	if m.FromNode != nil && m.FromNode.Connected {
		return ""
	}

	snr := ""
	if m.Snr != 0 {
		snr = fmt.Sprintf("SNR %.2f, ", m.Snr)
	}
	return fmt.Sprintf(
		"\033[90m(%s%d %s away)\033[0m",
		snr,
		m.HopsAway,
		helpers.Pluralize("hop", int(m.HopsAway)),
	)
}
