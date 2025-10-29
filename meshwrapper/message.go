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

type Message struct {
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

func (m *Message) ingestMeshPacket(connectedNode *ConnectedNode, meshPacket *meshtastic.MeshPacket) {
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
		MessageEvents.publish(NodeInfoEvent, *m)

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
			MessageEvents.publish(DeviceTelemetryEvent, *m)
		case *meshtastic.Telemetry_EnvironmentMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_ENVIRONMENT
			m.EnvironmentMetrics = result.GetEnvironmentMetrics()
			MessageEvents.publish(EnvironmentTelemetryEvent, *m)
		case *meshtastic.Telemetry_HealthMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_HEALTH
			m.HealthMetrics = result.GetHealthMetrics()
			MessageEvents.publish(HealthTelemetryEvent, *m)
		case *meshtastic.Telemetry_AirQualityMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_AIR_QUALITY
			m.AirQualityMetrics = result.GetAirQualityMetrics()
			MessageEvents.publish(AirQualityTelemetryEvent, *m)
		case *meshtastic.Telemetry_PowerMetrics:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_POWER
			m.PowerMetrics = result.GetPowerMetrics()
			MessageEvents.publish(PowerTelemetryEvent, *m)
		case *meshtastic.Telemetry_LocalStats:
			m.MessageType = MESSAGE_TYPE_TELEMETRY_LOCAL_STATS
			m.LocalStats = result.GetLocalStats()
			MessageEvents.publish(LocalStatsTelemetryEvent, *m)
		default:
			log.Println("Warning: Unknown telemetry variant:", result.String())
		}
		MessageEvents.publish(TelemetryEvent, *m)

	case meshtastic.PortNum_POSITION_APP:
		result := meshtastic.Position{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall Position mesh packet: " + err.Error())
			return
		}
		m.MessageType = MESSAGE_TYPE_POSITION
		m.Position = NewPosition(&result)
		MessageEvents.publish(PositionEvent, *m)

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
		MessageEvents.publish(NeighborInfoEvent, *m)

	case meshtastic.PortNum_TEXT_MESSAGE_APP:
		m.MessageType = MESSAGE_TYPE_TEXT_MESSAGE
		m.Text = string(payload)
		MessageEvents.publish(TextMessageEvent, *m)

	case meshtastic.PortNum_ROUTING_APP:
		if meshPacket.GetDecoded() != nil {
			result := meshtastic.Routing{}
			err := proto.Unmarshal(payload, &result)
			if err != nil {
				log.Println("Error: Could not unmarshall Routing mesh packet: " + err.Error())
				return
			}
			if result.GetErrorReason() != meshtastic.Routing_NONE {
				log.Println("Bad acknowledgement: " + meshtastic.Routing_Error_name[int32(result.GetErrorReason())])
			}
			messageId := meshPacket.GetDecoded().RequestId
			if connectedNode.Acks[messageId] != nil {
				connectedNode.Acks[messageId] <- result.GetErrorReason() == meshtastic.Routing_NONE
				close(connectedNode.Acks[messageId])
				delete(connectedNode.Acks, messageId)
			}
		}
		m.MessageType = MESSAGE_TYPE_ROUTING
		MessageEvents.publish(RoutingEvent, *m)

	case meshtastic.PortNum_TRACEROUTE_APP:
		m.MessageType = MESSAGE_TYPE_TRACEROUTE
		MessageEvents.publish(TraceRouteEvent, *m)

	default:
		log.Println("Warning: Unknown mesh packet:", meshPacket.String())

	}
}

func (m Message) ReplyReliably(message string, retries ...int) chan bool {
	ch := make(chan bool)
	attempt := 1
	maxAttempts := 3
	if len(retries) > 0 {
		maxAttempts = retries[0]
	}
	go func() {
		delivered := false
		for {
			log.Printf("Attempt %d to send message...\n", attempt)
			delivered = <-m.Reply(message)
			if delivered {
				log.Println("Delivered successfully")
				break
			}
			if attempt == maxAttempts {
				log.Printf("Made %d attempts to send message, aborting\n", attempt)
				break
			}
			attempt++
		}
		ch <- delivered
		close(ch)
	}()
	return ch
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
	id, err := m.sendTextMessage(message)
	if err != nil {
		log.Println("Could not send message:", err)
		ch <- false
		close(ch)
		return ch
	}
	m.ReceivingNode.Acks[id] = ch
	go func() {
		time.Sleep(timeout)
		if m.ReceivingNode.Acks[id] != nil {
			ch <- false
			close(ch)
			delete(m.ReceivingNode.Acks, id)
		}
	}()
	return ch
}

func (m *Message) sendTextMessage(message string) (uint32, error) {
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
	msg := Message{
		FromNode:    m.ReceivingNode.Node,
		ToNode:      recipient,
		Text:        message,
		MessageType: MESSAGE_TYPE_TEXT_MESSAGE,
		Timestamp:   time.Now(),
		Channel:     m.Channel,
	}
	MessageEvents.publish(OutgoingMessageEvent, msg)

	// Actually send the message
	return m.ReceivingNode.SendMessage(channelId, recipient, message, min(m.HopsAway+2, 7))
}

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
	if m.Channel == nil {
		return "UNKNOWN"
	}
	return m.Channel.name
}

func (m Message) GetSenderNode() *Node {
	return m.FromNode
}

func (m Message) GetReceiverNode() *Node {
	return m.ToNode
}

func (m Message) FindNode(needle string) *Node {
	if m.ReceivingNode == nil {
		return nil
	}
	return m.ReceivingNode.NodeList.findNode(needle)
}

func (m Message) String() string {
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

	var content string
	if m.MessageType == MESSAGE_TYPE_TEXT_MESSAGE {
		content = m.Text
	} else {
		content = "\033[1m" + m.MessageType + " packet\033[0m"
	}

	return fmt.Sprintf("%s: %s %s", direction, content, m.radioMetricsString())
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
