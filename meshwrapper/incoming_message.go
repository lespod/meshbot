package meshwrapper

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"github.com/timendus/meshbot/meshwrapper/helpers"
	"google.golang.org/protobuf/proto"
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

type IncomingMessage struct {
	FromNode      *Node
	ToNode        *Node
	ReceivingNode *ConnectedNode

	Timestamp time.Time
	Snr       float32
	HopsAway  uint32

	MessageType        string
	Text               string
	Channel            *Channel
	DeviceMetrics      *meshtastic.DeviceMetrics
	EnvironmentMetrics *meshtastic.EnvironmentMetrics
	HealthMetrics      *meshtastic.HealthMetrics
	AirQualityMetrics  *meshtastic.AirQualityMetrics
	PowerMetrics       *meshtastic.PowerMetrics
	UserInfo           *meshtastic.User
	LocalStats         *meshtastic.LocalStats
	NeighborInfo       *meshtastic.NeighborInfo
	Position           *Position
}

func (m *IncomingMessage) ingestMeshPacket(connectedNode *ConnectedNode, meshPacket *meshtastic.MeshPacket) {
	if meshPacket.HopStart == 0 {
		m.HopsAway = 0
	} else {
		m.HopsAway = meshPacket.HopStart - meshPacket.HopLimit
	}

	payload := meshPacket.GetDecoded().GetPayload()
	switch meshPacket.GetDecoded().Portnum {

	case meshtastic.PortNum_NODEINFO_APP:
		result := meshtastic.User{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall NodeInfo User mesh packet: " + err.Error())
			return
		}
		m.MessageType = MESSAGE_TYPE_NODE_INFO
		m.UserInfo = &result
		IncomingMessageEvents.publish(NodeInfoEvent, *m)

	case meshtastic.PortNum_TELEMETRY_APP:
		result := meshtastic.Telemetry{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall Telemetry mesh packet: " + err.Error())
			return
		}
		switch result.Variant.(type) {
		case *meshtastic.Telemetry_DeviceMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_DEVICE
			m.DeviceMetrics = result.GetDeviceMetrics()
			IncomingMessageEvents.publish(DeviceTelemetryEvent, *m)
		case *meshtastic.Telemetry_EnvironmentMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_ENVIRONMENT
			m.EnvironmentMetrics = result.GetEnvironmentMetrics()
			IncomingMessageEvents.publish(EnvironmentTelemetryEvent, *m)
		case *meshtastic.Telemetry_HealthMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_HEALTH
			m.HealthMetrics = result.GetHealthMetrics()
			IncomingMessageEvents.publish(HealthTelemetryEvent, *m)
		case *meshtastic.Telemetry_AirQualityMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_AIR_QUALITY
			m.AirQualityMetrics = result.GetAirQualityMetrics()
			IncomingMessageEvents.publish(AirQualityTelemetryEvent, *m)
		case *meshtastic.Telemetry_PowerMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_POWER
			m.PowerMetrics = result.GetPowerMetrics()
			IncomingMessageEvents.publish(PowerTelemetryEvent, *m)
		case *meshtastic.Telemetry_LocalStats:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_LOCAL_STATS
			m.LocalStats = result.GetLocalStats()
			IncomingMessageEvents.publish(LocalStatsTelemetryEvent, *m)
		default:
			log.Println("Warning: Unknown telemetry variant:", result.String())
		}
		IncomingMessageEvents.publish(TelemetryEvent, *m)

	case meshtastic.PortNum_POSITION_APP:
		result := meshtastic.Position{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall Position mesh packet: " + err.Error())
			return
		}
		m.MessageType = MESSAGE_TYPE_POSITION
		m.Position = NewPosition(&result)
		IncomingMessageEvents.publish(PositionEvent, *m)

	case meshtastic.PortNum_NEIGHBORINFO_APP:
		result := meshtastic.NeighborInfo{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall NeighborInfo mesh packet: " + err.Error())
			return
		}
		m.MessageType = MESSAGE_TYPE_NEIGHBOR_INFO
		m.NeighborInfo = &result
		helpers.Assert(result.NodeId == meshPacket.From, "I don't understand this format well enough: received "+m.String()+" but it has NodeId "+strconv.Itoa(int(result.NodeId)))
		IncomingMessageEvents.publish(NeighborInfoEvent, *m)

	case meshtastic.PortNum_TEXT_MESSAGE_APP:
		m.MessageType = MESSAGE_TYPE_TEXT_MESSAGE
		m.Text = string(payload)
		IncomingMessageEvents.publish(TextMessageEvent, *m)

	case meshtastic.PortNum_ROUTING_APP:
		if meshPacket.GetDecoded() != nil {
			result := meshtastic.Routing{}
			err := proto.Unmarshal(payload, &result)
			if err != nil {
				log.Println("Error: Could not unmarshall Routing mesh packet: " + err.Error())
				return
			}
			messageId := meshPacket.GetDecoded().RequestId
			ack, ok := connectedNode.Acks[messageId]
			if ok {
				ack.receive(m.FromNode, result.GetErrorReason())
				delete(connectedNode.Acks, messageId)
			}
		}
		m.MessageType = MESSAGE_TYPE_ROUTING
		IncomingMessageEvents.publish(RoutingEvent, *m)

	case meshtastic.PortNum_TRACEROUTE_APP:
		m.MessageType = MESSAGE_TYPE_TRACEROUTE
		IncomingMessageEvents.publish(TraceRouteEvent, *m)

	default:
		log.Println("Warning: Unknown mesh packet:", meshPacket.String())

	}
}

func (m IncomingMessage) ReplyReliably(message string, retries ...int) chan bool {
	ch := make(chan bool)
	messageTimeout := DEFAULT_BLOCKING_MESSAGE_TIMEOUT

	maxAttempts := 3
	if len(retries) > 0 {
		maxAttempts = retries[0]
	}

	go func() {
		for _, msg := range helpers.BreakMessage(message) {
			attempt := 1
			delivered := false
			for attempt <= maxAttempts {
				ack := m.send(msg, messageTimeout)
				delivered = <-ack.delivered
				if delivered {
					break
				}
				attempt++
			}
			if !delivered {
				// Failed to deliver at least part of the message, abort
				ch <- false
				close(ch)
				return
			}
		}

		// Made it through all parts of the message successfully
		ch <- true
		close(ch)
	}()

	return ch
}

func (m IncomingMessage) Reply(message string, timeout ...time.Duration) chan bool {
	ch := make(chan bool)

	go func() {
		messageTimeout := DEFAULT_BLOCKING_MESSAGE_TIMEOUT
		if len(timeout) > 0 {
			messageTimeout = timeout[0]
		}

		for _, msg := range helpers.BreakMessage(message) {
			ack := m.send(msg, messageTimeout)
			delivered := <-ack.delivered
			if !delivered {
				ch <- false
				return
			}
		}

		ch <- true
	}()

	return ch
}

func (m *IncomingMessage) send(message string, timeout time.Duration) *acknowledgement {
	ack := newAcknowledgement(m.FromNode)
	id, err := m.sendTextMessage(message)
	if err != nil {
		// Give user feedback, also when acknowledgements are not verbose,
		// because there's a good chance that the error we get here is due to
		// the user's configuration choices.
		log.Println("Could not send message:", err)
		ack.error(err)
		return ack
	}
	m.ReceivingNode.Acks[id] = ack
	go func() {
		time.Sleep(timeout)
		ack.timeout()
		delete(m.ReceivingNode.Acks, id)
	}()
	return ack
}

func (m *IncomingMessage) sendTextMessage(message string) (uint32, error) {
	helpers.Assert(m.ReceivingNode != nil, "Can't send a message without knowing through which device to send it")
	helpers.Assert(m.FromNode != nil, "Can't send a message to an unknown node")
	helpers.Assert(m.ToNode != nil, "Can't send a message from an unknown node")

	// If message was sent to a channel, reply in the same channel instead of
	// privately.
	recipient := m.FromNode
	if m.ToNode.Id == Broadcast.Id {
		recipient = &Broadcast
	}
	channelId := uint32(0)
	if m.Channel != nil {
		channelId = m.Channel.id
	}

	// Notify the rest of the system that we're sending this message
	msg := IncomingMessage{
		FromNode:    m.ReceivingNode.Node,
		ToNode:      recipient,
		Text:        message,
		MessageType: MESSAGE_TYPE_TEXT_MESSAGE,
		Timestamp:   time.Now(),
		Channel:     m.Channel,
	}
	IncomingMessageEvents.publish(OutgoingMessageEvent, msg)

	// Actually send the message
	return m.ReceivingNode.SendMessage(channelId, recipient, message, min(m.HopsAway+2, 7))
}

func (m IncomingMessage) GetText() string {
	return m.Text
}

func (m IncomingMessage) IsPrivateMessage() bool {
	return m.ToNode != nil && m.ToNode.Id != Broadcast.Id
}

func (m IncomingMessage) GetType() string {
	return m.MessageType
}

func (m IncomingMessage) GetChannelName() string {
	if m.Channel == nil {
		return "UNKNOWN"
	}
	return m.Channel.name
}

func (m IncomingMessage) GetSenderNode() *Node {
	return m.FromNode
}

func (m IncomingMessage) GetReceiverNode() *Node {
	return m.ToNode
}

func (m IncomingMessage) FindNode(needle string) *Node {
	if m.ReceivingNode == nil {
		return nil
	}
	return m.ReceivingNode.NodeList.findNode(needle)
}

func (m IncomingMessage) String() string {
	direction := ""
	if m.FromNode != nil {
		direction += m.FromNode.ColorString()
	} else {
		direction += "No node"
	}
	if m.ToNode != nil {
		direction += " -> " + m.ToNode.ColorString()
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

	if m.MessageType == MESSAGE_TYPE_TEXT_MESSAGE {
		return fmt.Sprintf("%s: %s\n%s", direction, m.radioMetricsString(), helpers.Indent(m.Text, "\t"))
	}

	return fmt.Sprintf("%s: \033[1m%s packet\033[0m %s", direction, m.MessageType, m.radioMetricsString())
}

func (m *IncomingMessage) radioMetricsString() string {
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
