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
	MESSAGE_TYPE_TEXT_MESSAGE          = "wiadomość tekstowa"
	MESSAGE_TYPE_NODE_INFO             = "informacje o nodzie"
	MESSAGE_TYPE_POSITION              = "pozycja"
	MESSAGE_TYPE_NEIGHBOR_INFO         = "informacje o sąsiadach"
	MESSAGE_TYPE_ROUTING               = "routing"
	MESSAGE_TYPE_TRACEROUTE            = "traceroute"
	MESSAGE_TYPE_TELEMETRY_DEVICE      = "telemetria urządzenia"
	MESSAGE_TYPE_TELEMETRY_ENVIRONMENT = "telemetria środowiskowa"
	MESSAGE_TYPE_TELEMETRY_HEALTH      = "telemetria zdrowia"
	MESSAGE_TYPE_TELEMETRY_AIR_QUALITY = "telemetria jakości powietrza"
	MESSAGE_TYPE_TELEMETRY_POWER       = "telemetria zasilania"
	MESSAGE_TYPE_TELEMETRY_LOCAL_STATS = "lokalne statystyki telemetryczne"
	MESSAGE_TYPE_OTHER                 = "inne"

	DEFAULT_BLOCKING_MESSAGE_TIMEOUT = 60 * time.Second
)

type IncomingMessage struct {
	FromNode      *Node
	ToNode        *Node
	ReceivingNode *ConnectedNode

	Timestamp time.Time
	Snr       float32
	HopsAway  uint32
	PacketId  uint32
	RequestId uint32

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
	RouteDiscovery     *meshtastic.RouteDiscovery
	Position           *Position
}

func (m *IncomingMessage) ingestMeshPacket(connectedNode *ConnectedNode, meshPacket *meshtastic.MeshPacket) {
	if meshPacket.HopStart == 0 {
		m.HopsAway = 0
	} else {
		m.HopsAway = meshPacket.HopStart - meshPacket.HopLimit
	}

	payload := meshPacket.GetDecoded().GetPayload()
	m.RequestId = meshPacket.GetDecoded().GetRequestId()
	switch meshPacket.GetDecoded().Portnum {

	case meshtastic.PortNum_NODEINFO_APP:
		result := meshtastic.User{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Błąd: nie udało się odkodować pakietu NodeInfo User: " + err.Error())
			return
		}
		m.MessageType = MESSAGE_TYPE_NODE_INFO
		m.UserInfo = &result
		IncomingMessageEvents.publish(NodeInfoEvent, *m)

	case meshtastic.PortNum_TELEMETRY_APP:
		result := meshtastic.Telemetry{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Błąd: nie udało się odkodować pakietu Telemetry: " + err.Error())
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
			log.Println("Ostrzeżenie: nieznany wariant telemetrii:", result.String())
		}
		IncomingMessageEvents.publish(TelemetryEvent, *m)

	case meshtastic.PortNum_POSITION_APP:
		result := meshtastic.Position{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Błąd: nie udało się odkodować pakietu Position: " + err.Error())
			return
		}
		m.MessageType = MESSAGE_TYPE_POSITION
		m.Position = NewPosition(&result)
		IncomingMessageEvents.publish(PositionEvent, *m)

	case meshtastic.PortNum_NEIGHBORINFO_APP:
		result := meshtastic.NeighborInfo{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Błąd: nie udało się odkodować pakietu NeighborInfo: " + err.Error())
			return
		}
		m.MessageType = MESSAGE_TYPE_NEIGHBOR_INFO
		m.NeighborInfo = &result
		helpers.Assert(result.NodeId == meshPacket.From, "Nie rozumiem jeszcze tego formatu wystarczająco dobrze: odebrano "+m.String()+" z NodeId "+strconv.Itoa(int(result.NodeId)))
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
				log.Println("Błąd: nie udało się odkodować pakietu Routing: " + err.Error())
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
		result := meshtastic.RouteDiscovery{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Błąd: nie udało się odkodować pakietu RouteDiscovery: " + err.Error())
			return
		}
		m.MessageType = MESSAGE_TYPE_TRACEROUTE
		m.RouteDiscovery = &result
		IncomingMessageEvents.publish(TraceRouteEvent, *m)

	default:
		log.Println("Ostrzeżenie: nieznany pakiet mesh:", meshPacket.String())

	}
}

func (m *IncomingMessage) Reply(message string) chan bool {
	return m.newOutgoingMessage(message).Send()
}

func (m *IncomingMessage) ReplyTo(message string) chan bool {
	outgoing := m.newOutgoingMessage(message)
	outgoing.ReplyId = m.PacketId
	return outgoing.Send()
}

func (m *IncomingMessage) ReplyReliably(message string) chan bool {
	return m.newOutgoingMessage(message).SendReliably()
}

func (m *IncomingMessage) newOutgoingMessage(message string) *OutgoingMessage {
	hops := min(int(m.HopsAway)+2, 7)
	if m.IsPrivateMessage() {
		return NewOutgoingDirectMessage(message, m.ReceivingNode, m.FromNode, hops)
	} else {
		return NewOutgoingChannelMessage(message, m.ReceivingNode, m.Channel, hops)
	}
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
		return "NIEZNANY"
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
		direction += "Brak noda"
	}
	if m.IsPrivateMessage() {
		if m.ToNode != nil {
			direction += " -> " + m.ToNode.ColorString()
		} else {
			direction += " -> Brak noda"
		}
	} else {
		if m.Channel != nil {
			direction += " -> Kanał " + m.Channel.name
		} else {
			direction += " -> Nieznany kanał"
		}
	}

	if m.MessageType == MESSAGE_TYPE_NEIGHBOR_INFO {
		neighbours := "nieznani"
		if m.FromNode != nil {
			neighbours = m.FromNode.Neighbors.String()
		}
		return fmt.Sprintf("%s: \033[1mLista sąsiadów:\033[0m %s %s", direction, m.radioMetricsString(), neighbours)
	}

	if m.MessageType == MESSAGE_TYPE_TEXT_MESSAGE {
		return fmt.Sprintf("%s: %s\n%s", direction, m.radioMetricsString(), helpers.Indent(m.Text, "\t"))
	}

	return fmt.Sprintf("%s: \033[1mPakiet: %s\033[0m %s", direction, m.MessageType, m.radioMetricsString())
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
		"\033[90m(%s%d %s stąd)\033[0m",
		snr,
		m.HopsAway,
		helpers.PolishHopWord(int(m.HopsAway)),
	)
}
