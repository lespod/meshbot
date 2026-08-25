package meshwrapper

import (
	"fmt"
	"log"
	"time"

	"github.com/timendus/meshbot/meshwrapper/helpers"
)

const DEFAULT_DELIVERY_TIMEOUT = 60 * time.Second

type OutgoingMessage struct {
	FromNode      *Node
	ToNode        *Node
	Channel       *Channel
	ReceivingNode *ConnectedNode
	Text          string

	MaxHops            int
	Retries            int
	Timeout            time.Duration
	ReplyId            uint32
	CurrentMessagePart string
}

func NewOutgoingDirectMessage(message string, from *ConnectedNode, to *Node, hops int) *OutgoingMessage {
	return &OutgoingMessage{
		FromNode:      from.Node,
		ToNode:        to,
		ReceivingNode: from,
		Text:          message,

		MaxHops: hops,
		Retries: 3,
		Timeout: DEFAULT_DELIVERY_TIMEOUT,
	}
}

func NewOutgoingChannelMessage(message string, from *ConnectedNode, to *Channel, hops int) *OutgoingMessage {
	return &OutgoingMessage{
		FromNode:      from.Node,
		ToNode:        &Broadcast,
		Channel:       to,
		ReceivingNode: from,
		Text:          message,

		MaxHops: hops,
		Retries: 3,
		Timeout: DEFAULT_DELIVERY_TIMEOUT,
	}
}

// Zwykła wysyłka.
func (m *OutgoingMessage) Send() chan bool {
	helpers.Assert(m.ReceivingNode != nil, "Nie można wysłać wiadomości bez urządzenia wysyłającego")
	helpers.Assert(m.FromNode != nil, "Nie można wysłać wiadomości do nieznanego noda")
	helpers.Assert(m.ToNode != nil, "Nie można wysłać wiadomości od nieznanego noda")

	ch := make(chan bool)

	go func() {
		for _, msg := range helpers.BreakMessage(m.Text) {
			ack := m.send(msg)
			delivered := m.delivered(ack)
			if !delivered {
				ch <- false
				return
			}
		}

		ch <- true
	}()

	return ch
}

// Wysyłka z ponowieniami po błędzie doręczenia.
func (m *OutgoingMessage) SendReliably() chan bool {
	helpers.Assert(m.ReceivingNode != nil, "Nie można wysłać wiadomości bez urządzenia wysyłającego")
	helpers.Assert(m.FromNode != nil, "Nie można wysłać wiadomości do nieznanego noda")
	helpers.Assert(m.ToNode != nil, "Nie można wysłać wiadomości od nieznanego noda")

	ch := make(chan bool)

	go func() {
		for _, msg := range helpers.BreakMessage(m.Text) {
			attempt := 1
			delivered := false
			for attempt <= m.Retries {
				ack := m.send(msg)
				delivered = m.delivered(ack)
				if delivered {
					break
				}
				attempt++
			}
			if !delivered {
				// Nie udało się doręczyć co najmniej jednej części, więc przerywamy.
				ch <- false
				close(ch)
				return
			}
		}

		// Wszystkie części wiadomości zostały obsłużone.
		ch <- true
		close(ch)
	}()

	return ch
}

func (m *OutgoingMessage) send(message string) *acknowledgement {
	var channelId uint32
	if m.isPrivateMessage() {
		channelId = 0
	} else {
		channelId = m.Channel.id
	}

	// Wyślij wiadomość.
	ack := newAcknowledgement(m.ToNode)
	id, err := m.ReceivingNode.SendMessage(channelId, m.ToNode, message, uint32(m.MaxHops), m.ReplyId)
	if err != nil {
		// Loguj też przy cichych potwierdzeniach, bo częstą przyczyną jest konfiguracja.
		log.Println("Nie udało się wysłać wiadomości:", err)
		ack.error(err)
		return ack
	}
	m.ReceivingNode.Acks[id] = ack

	// Uruchom timeout potwierdzenia.
	go func() {
		time.Sleep(m.Timeout)
		ack.timeout()
		delete(m.ReceivingNode.Acks, id)
	}()

	// Powiadom resztę systemu o wysłanej wiadomości.
	m.CurrentMessagePart = message
	OutgoingMessageEvents.publish(OutgoingMessageEvent, *m)

	return ack
}

func (m *OutgoingMessage) isPrivateMessage() bool {
	helpers.Assert(m.ToNode != nil, "ToNode powinien zostać sprawdzony wcześniej")
	return m.ToNode.Id != Broadcast.Id
}

func (m *OutgoingMessage) delivered(ack *acknowledgement) bool {
	if m.isPrivateMessage() {
		// Wiadomość prywatna jest doręczona, gdy dotrze do odbiorcy.
		return <-ack.delivered
	} else {
		// Wiadomość kanałowa jest doręczona, gdy dotrze do celu albo usłyszymy jej powtórzenie.
		select {
		case delivered := <-ack.delivered:
			return delivered
		case delivered := <-ack.repeated:
			return delivered
		}
	}
}

func (m *OutgoingMessage) String() string {
	helpers.Assert(m.FromNode != nil, "FromNode powinien być już znany")
	helpers.Assert(m.ToNode != nil, "ToNode powinien być już znany")

	direction := m.FromNode.ColorString()
	if m.isPrivateMessage() {
		direction += " -> " + m.ToNode.ColorString()
	} else {
		direction += " -> Kanał " + m.Channel.name
	}

	contents := m.CurrentMessagePart
	if contents == "" {
		contents = m.Text
	}
	return fmt.Sprintf("%s:\n%s", direction, helpers.Indent(contents, "\t"))
}
