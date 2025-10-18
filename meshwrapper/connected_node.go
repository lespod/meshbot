package meshwrapper

import (
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"strconv"
	"time"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"github.com/timendus/meshbot/config"
	"github.com/timendus/meshbot/meshwrapper/helpers"
	"google.golang.org/protobuf/proto"
)

type ConnectedNode struct {
	aquireStream    func() (io.ReadWriteCloser, error)
	stream          io.ReadWriteCloser
	Connected       bool
	FirmwareVersion string
	Channels        map[uint32]Channel
	Node            *Node
	NodeList        nodeList
	Acks            map[uint32]chan bool
}

func NewConnectedNode(aquire func() (io.ReadWriteCloser, error)) *ConnectedNode {
	return &ConnectedNode{
		aquireStream: aquire,
		Connected:    false,
		NodeList:     NewNodeList(),
		Acks:         make(map[uint32]chan bool),
		Channels:     make(map[uint32]Channel),
		Node: &Node{
			ShortName: "UNKN",
			LongName:  "Unknown node",
			Id:        0,
			Connected: true,
		},
	}
}

func (n *ConnectedNode) Connect() error {
	// Connect to the actual device
	stream, err := n.aquireStream()
	if err != nil {
		return err
	}
	n.stream = stream

	// Spin up a goroutine to read messages from the device
	go n.readMessages(n.stream)

	// Wake the device
	if err := wakeDevice(n.stream); err != nil {
		return err
	}

	// Tell the device that we can speak ProtoBuf
	if err := writeMessage(n.stream, &meshtastic.ToRadio{
		PayloadVariant: &meshtastic.ToRadio_WantConfigId{
			WantConfigId: 1,
		},
	}); err != nil {
		return err
	}
	return nil
}

func (n *ConnectedNode) Close() error {
	n.Connected = false
	ConnectionEvents.publish(DisconnectedEvent, *n)
	return n.stream.Close()
}

func (n *ConnectedNode) String() string {
	return n.Node.ColorString()
}

func (n *ConnectedNode) SendMessage(channel uint32, recipient *Node, message string, hopLimit uint32) (uint32, error) {
	id := rand.Uint32()
	err := n.SendPacket(meshtastic.ToRadio_Packet{
		Packet: &meshtastic.MeshPacket{
			Id:       id,
			Channel:  channel,
			To:       recipient.Id,
			From:     n.Node.Id,
			HopLimit: hopLimit,
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
	return id, err
}

func (n *ConnectedNode) SendPacket(message meshtastic.ToRadio_Packet) error {
	// Only transmit anything if the configuration allows it or the
	// configuration has this particular node id as the exception. Otherwise,
	// just silently drop the transmission.
	cfg := config.GetConfig()
	nodeAllowed := cfg.Settings.TransmitExceptionNodeId != 0 && message.Packet.To == cfg.Settings.TransmitExceptionNodeId
	if !(cfg.Settings.AllowTransmit || nodeAllowed) {
		return fmt.Errorf("not allowed to transmit by config.json")
	}

	// If message is a message in a channel, but the configuration does not
	// allow this, again just drop the transmission
	if !cfg.Settings.AllowTransmitToChannels && message.Packet.To == Broadcast.Id {
		return fmt.Errorf("not allowed to transmit in a channel by config.json")
	}

	if err := writeMessage(n.stream, &meshtastic.ToRadio{
		PayloadVariant: &message,
	}); err != nil {
		return err
	}
	return nil
}

func (n *ConnectedNode) readMessages(stream io.ReadCloser) error {
	for {
		packet, err := readMessage(stream)
		if err != nil {
			log.Println("Error: " + err.Error())
			if err == io.EOF {
				log.Println("EOF probably means the device has disconnected. Stopping execution.")
				return n.Close()
			}
			continue
		}

		switch packet.PayloadVariant.(type) {
		case *meshtastic.FromRadio_ConfigCompleteId:
			n.Connected = true
			ConnectionEvents.publish(ConnectedEvent, *n)
		case *meshtastic.FromRadio_MyInfo:
			n.Node.Id = packet.GetMyInfo().MyNodeNum
			n.NodeList.nodes[n.Node.Id] = n.Node
		case *meshtastic.FromRadio_Metadata:
			n.FirmwareVersion = packet.GetMetadata().FirmwareVersion
		case *meshtastic.FromRadio_NodeInfo:
			n.parseNodeInfo(packet.GetNodeInfo())
		case *meshtastic.FromRadio_Channel:
			channel := NewChannel(packet.GetChannel())
			n.Channels[channel.id] = channel
		case *meshtastic.FromRadio_Packet:
			n.parseMeshPacket(packet.GetPacket())
		case *meshtastic.FromRadio_Config:
		case *meshtastic.FromRadio_ModuleConfig:
		case *meshtastic.FromRadio_FileInfo:
		case *meshtastic.FromRadio_QueueStatus:
			// Silently ignore these packets
		default:
			log.Println("Unhandled message:" + packet.String())
		}
	}
}

func (n *ConnectedNode) parseNodeInfo(nodeInfo *meshtastic.NodeInfo) {
	// Create or update the node that this info relates to
	relevantNode, exists := n.NodeList.nodes[nodeInfo.Num]
	if !exists {
		n.NodeList.nodes[nodeInfo.Num] = NewNode(nodeInfo)
	} else {
		relevantNode.Update(nodeInfo)
	}
}

func (n *ConnectedNode) parseMeshPacket(meshPacket *meshtastic.MeshPacket) {
	// Ignore broken, encrypted or empty packets
	if meshPacket == nil || meshPacket.GetDecoded() == nil || meshPacket.GetDecoded().GetPayload() == nil {
		return
	}

	var hops uint32
	if meshPacket.HopStart == 0 {
		hops = 0
	} else {
		hops = meshPacket.HopStart - meshPacket.HopLimit
	}

	payload := meshPacket.GetDecoded().GetPayload()

	toNode := n.NodeList.nodes[meshPacket.To]
	fromNode := n.NodeList.nodes[meshPacket.From]

	if fromNode == nil {
		// If the sending node is not in our node list yet, just add it.
		fromNode = NewNode(&meshtastic.NodeInfo{
			Num:       meshPacket.From,
			LastHeard: meshPacket.RxTime,
		})
		n.NodeList.nodes[meshPacket.From] = fromNode
	}

	fromNode.HopsAway = hops
	if hops == 0 {
		// Assumption: the packet RxSnr is the signal quality of the received
		// packet, which may have hopped through other nodes. So only update
		// this node's SNR if we haven't hopped yet.
		fromNode.Snr = meshPacket.RxSnr
	}

	channel := n.Channels[meshPacket.Channel]

	message := Message{
		FromNode:      fromNode,
		ToNode:        toNode,
		ReceivingNode: n,
		Channel:       &channel,
		Timestamp:     time.Unix(int64(meshPacket.RxTime), 0),
		MessageType:   MESSAGE_TYPE_OTHER,
		Snr:           meshPacket.RxSnr,
		HopsAway:      hops,
	}

	fromNode.ReceivedMessages = append(fromNode.ReceivedMessages, &message)

	switch meshPacket.GetDecoded().Portnum {
	case meshtastic.PortNum_NODEINFO_APP:
		result := meshtastic.User{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall NodeInfo User mesh packet: " + err.Error())
			return
		}
		fromNode.ShortName = result.ShortName
		fromNode.LongName = result.LongName
		fromNode.HwModel = result.HwModel
		fromNode.Role = result.Role
		fromNode.IsLicensed = result.IsLicensed
		fromNode.PublicKey = result.PublicKey
		message.MessageType = MESSAGE_TYPE_NODE_INFO
		MessageEvents.publish(NodeInfoEvent, message)

	case meshtastic.PortNum_TELEMETRY_APP:
		result := meshtastic.Telemetry{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall Telemetry mesh packet: " + err.Error())
			return
		}
		switch result.Variant.(type) {
		case *meshtastic.Telemetry_DeviceMetrics:
			message.MessageType = MESSAGE_TYPE_TELEMETRY_DEVICE
			message.DeviceMetrics = result.GetDeviceMetrics()
			MessageEvents.publish(DeviceTelemetryEvent, message)
		case *meshtastic.Telemetry_EnvironmentMetrics:
			message.MessageType = MESSAGE_TYPE_TELEMETRY_ENVIRONMENT
			message.EnvironmentMetrics = result.GetEnvironmentMetrics()
			MessageEvents.publish(EnvironmentTelemetryEvent, message)
		case *meshtastic.Telemetry_HealthMetrics:
			message.MessageType = MESSAGE_TYPE_TELEMETRY_HEALTH
			message.HealthMetrics = result.GetHealthMetrics()
			MessageEvents.publish(HealthTelemetryEvent, message)
		case *meshtastic.Telemetry_AirQualityMetrics:
			message.MessageType = MESSAGE_TYPE_TELEMETRY_AIR_QUALITY
			message.AirQualityMetrics = result.GetAirQualityMetrics()
			MessageEvents.publish(AirQualityTelemetryEvent, message)
		case *meshtastic.Telemetry_PowerMetrics:
			message.MessageType = MESSAGE_TYPE_TELEMETRY_POWER
			message.PowerMetrics = result.GetPowerMetrics()
			MessageEvents.publish(PowerTelemetryEvent, message)
		case *meshtastic.Telemetry_LocalStats:
			message.MessageType = MESSAGE_TYPE_TELEMETRY_LOCAL_STATS
			message.LocalStats = result.GetLocalStats()
			MessageEvents.publish(LocalStatsTelemetryEvent, message)
		default:
			log.Println("Warning: Unknown telemetry variant:", result.String())
		}
		MessageEvents.publish(TelemetryEvent, message)

	case meshtastic.PortNum_POSITION_APP:
		result := meshtastic.Position{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall Position mesh packet: " + err.Error())
			return
		}
		message.MessageType = MESSAGE_TYPE_POSITION
		message.Position = NewPosition(&result)
		MessageEvents.publish(PositionEvent, message)

	case meshtastic.PortNum_NEIGHBORINFO_APP:
		result := meshtastic.NeighborInfo{}
		err := proto.Unmarshal(payload, &result)
		if err != nil {
			log.Println("Error: Could not unmarshall NeighborInfo mesh packet: " + err.Error())
			return
		}
		message.MessageType = MESSAGE_TYPE_NEIGHBOR_INFO
		message.NeighborInfo = &result
		helpers.Assert(result.NodeId == meshPacket.From, "I don't understand this format well enough: received "+message.String()+" but it has NodeId "+strconv.Itoa(int(result.NodeId)))
		fromNode.Neighbors = NewNeighbourList(&n.NodeList, meshPacket.RxTime, result.Neighbors)
		MessageEvents.publish(NeighborInfoEvent, message)

	case meshtastic.PortNum_TEXT_MESSAGE_APP:
		message.MessageType = MESSAGE_TYPE_TEXT_MESSAGE
		message.Text = string(payload)
		MessageEvents.publish(TextMessageEvent, message)

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
			if n.Acks[messageId] != nil {
				n.Acks[messageId] <- result.GetErrorReason() == meshtastic.Routing_NONE
				close(n.Acks[messageId])
				delete(n.Acks, messageId)
			}
		}
		message.MessageType = MESSAGE_TYPE_ROUTING
		MessageEvents.publish(RoutingEvent, message)

	case meshtastic.PortNum_TRACEROUTE_APP:
		message.MessageType = MESSAGE_TYPE_TRACEROUTE
		MessageEvents.publish(TraceRouteEvent, message)

	default:
		log.Println("Warning: Unknown mesh packet:", meshPacket.String())
	}

	MessageEvents.publish(IncomingMessageEvent, message)
}
