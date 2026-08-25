package meshwrapper

import (
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"time"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"github.com/timendus/meshbot/config"
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
	Acks            map[uint32]*acknowledgement
}

func NewConnectedNode(aquire func() (io.ReadWriteCloser, error)) *ConnectedNode {
	return &ConnectedNode{
		aquireStream: aquire,
		Connected:    false,
		NodeList:     NewNodeList(),
		Acks:         make(map[uint32]*acknowledgement),
		Channels:     make(map[uint32]Channel),
		Node: &Node{
			ShortName: "UNKN",
			LongName:  "Nieznany node",
			Id:        0,
			Connected: true,
		},
	}
}

func (n *ConnectedNode) FindChannel(name string) (*Channel, bool) {
	for _, channel := range n.Channels {
		if channel.name == name {
			return &channel, true
		}
	}
	return nil, false
}

func (n *ConnectedNode) FindNodeById(id uint32) *Node {
	return n.NodeList.nodes[id]
}

func (n *ConnectedNode) Connect() error {
	// Połącz z właściwym urządzeniem.
	stream, err := n.aquireStream()
	if err != nil {
		return err
	}
	n.stream = stream

	// Uruchom goroutine czytającą wiadomości z urządzenia.
	go n.readMessages(n.stream)

	// Wybudź urządzenie.
	if err := wakeDevice(n.stream); err != nil {
		return err
	}

	// Poinformuj urządzenie, że obsługujemy ProtoBuf.
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

func (n *ConnectedNode) SendMessage(channel uint32, recipient *Node, message string, hopLimit uint32, replyId uint32) (uint32, error) {
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
					ReplyId: replyId,
				},
			},
		},
	})
	return id, err
}

func (n *ConnectedNode) SendTraceroute(recipient *Node, hopLimit uint32) (uint32, error) {
	if recipient == nil {
		return 0, fmt.Errorf("nieznany odbiorca traceroute")
	}
	id := rand.Uint32()
	payload, err := proto.Marshal(&meshtastic.RouteDiscovery{})
	if err != nil {
		return 0, err
	}

	err = n.SendPacket(meshtastic.ToRadio_Packet{
		Packet: &meshtastic.MeshPacket{
			Id:       id,
			Channel:  0,
			To:       recipient.Id,
			From:     n.Node.Id,
			HopLimit: hopLimit,
			WantAck:  true,
			Priority: meshtastic.MeshPacket_Priority(meshtastic.MeshPacket_Priority_value["RELIABLE"]),
			PayloadVariant: &meshtastic.MeshPacket_Decoded{
				Decoded: &meshtastic.Data{
					Portnum:      meshtastic.PortNum_TRACEROUTE_APP,
					Payload:      payload,
					WantResponse: true,
					Dest:         recipient.Id,
					Source:       n.Node.Id,
					RequestId:    id,
				},
			},
		},
	})
	return id, err
}

func (n *ConnectedNode) SendPacket(message meshtastic.ToRadio_Packet) error {
	// Nadawaj tylko wtedy, gdy konfiguracja na to pozwala albo wskazuje tego noda jako wyjątek.
	cfg := config.GetConfig()
	nodeAllowed := cfg.Settings.TransmitExceptionNodeId != 0 && message.Packet.To == cfg.Settings.TransmitExceptionNodeId
	if !(cfg.Settings.AllowTransmit || nodeAllowed) {
		return fmt.Errorf("nadawanie zablokowane przez config.json")
	}

	// Jeśli to wiadomość kanałowa, konfiguracja musi osobno pozwalać na nadawanie do kanałów.
	if !cfg.Settings.AllowTransmitToChannels && message.Packet.To == Broadcast.Id {
		return fmt.Errorf("nadawanie do kanału zablokowane przez config.json")
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
			log.Println("Błąd: " + err.Error())
			if err == io.EOF {
				log.Println("EOF prawdopodobnie oznacza rozłączenie urządzenia. Zatrzymuję obsługę połączenia.")
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
			channel := packet.GetChannel()
			if channel != nil && channel.Index >= 0 && channel.GetRole() != meshtastic.Channel_DISABLED {
				n.Channels[uint32(channel.Index)] = NewChannel(channel)
			}
		case *meshtastic.FromRadio_Packet:
			n.parseMeshPacket(packet.GetPacket())
		case *meshtastic.FromRadio_Config:
		case *meshtastic.FromRadio_ModuleConfig:
		case *meshtastic.FromRadio_FileInfo:
		case *meshtastic.FromRadio_QueueStatus:
			// Te pakiety ignorujemy świadomie.
		default:
			log.Println("Nieobsłużona wiadomość:" + packet.String())
		}
	}
}

func (n *ConnectedNode) parseNodeInfo(nodeInfo *meshtastic.NodeInfo) {
	// Utwórz albo zaktualizuj noda, którego dotyczy informacja.
	relevantNode, exists := n.NodeList.nodes[nodeInfo.Num]
	if !exists {
		n.NodeList.nodes[nodeInfo.Num] = NewNode(n, nodeInfo)
	} else {
		relevantNode.ingestNodeInfo(n, nodeInfo)
	}
}

func (n *ConnectedNode) parseMeshPacket(meshPacket *meshtastic.MeshPacket) {
	// Ignoruj uszkodzone albo zaszyfrowane pakiety.
	if meshPacket == nil || meshPacket.GetDecoded() == nil {
		return
	}

	fromNode, ok := n.NodeList.nodes[meshPacket.From]
	if !ok {
		fromNode = NewNode(n, &meshtastic.NodeInfo{
			Num: meshPacket.From,
		})
		n.NodeList.nodes[meshPacket.From] = fromNode
	}

	toNode, ok := n.NodeList.nodes[meshPacket.To]
	if !ok {
		toNode = NewNode(n, &meshtastic.NodeInfo{
			Num: meshPacket.To,
		})
		n.NodeList.nodes[meshPacket.To] = toNode
	}

	channel, ok := n.Channels[meshPacket.Channel]
	if !ok {
		channel = Channel{
			id:   meshPacket.Channel,
			name: "Nieznany",
		}
		n.Channels[meshPacket.Channel] = channel
	}

	message := IncomingMessage{
		FromNode:      fromNode,
		ToNode:        toNode,
		ReceivingNode: n,
		Channel:       &channel,
		Timestamp:     time.Unix(int64(meshPacket.RxTime), 0),
		MessageType:   MESSAGE_TYPE_OTHER,
		Snr:           meshPacket.RxSnr,
		PacketId:      meshPacket.Id,
	}
	message.ingestMeshPacket(n, meshPacket)
	fromNode.receiveMessage(n, message)

	IncomingMessageEvents.publish(IncomingMessageEvent, message)
}
