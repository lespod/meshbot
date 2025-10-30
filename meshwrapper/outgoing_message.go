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

	MaxHops int
	Retries int
	Timeout time.Duration
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

// Regular, boring old send
func (m *OutgoingMessage) Send() chan bool {
	helpers.Assert(m.ReceivingNode != nil, "Can't send a message without knowing through which device to send it")
	helpers.Assert(m.FromNode != nil, "Can't send a message to an unknown node")
	helpers.Assert(m.ToNode != nil, "Can't send a message from an unknown node")

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

// Send with retries on delivery failure
func (m *OutgoingMessage) SendReliably() chan bool {
	helpers.Assert(m.ReceivingNode != nil, "Can't send a message without knowing through which device to send it")
	helpers.Assert(m.FromNode != nil, "Can't send a message to an unknown node")
	helpers.Assert(m.ToNode != nil, "Can't send a message from an unknown node")

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

func (m *OutgoingMessage) send(message string) *acknowledgement {
	// Notify the rest of the system that we're sending this message
	OutgoingMessageEvents.publish(OutgoingMessageEvent, *m)

	var channelId uint32
	if m.isPrivateMessage() {
		channelId = 0
	} else {
		channelId = m.Channel.id
	}

	// Actually send the message
	ack := newAcknowledgement(m.FromNode)
	id, err := m.ReceivingNode.SendMessage(channelId, m.ToNode, message, uint32(m.MaxHops))
	if err != nil {
		// Give user feedback, also when acknowledgements are not verbose,
		// because there's a good chance that the error we get here is due to
		// the user's configuration choices.
		log.Println("Could not send message:", err)
		ack.error(err)
		return ack
	}
	m.ReceivingNode.Acks[id] = ack

	// Make the acknowledgement timeout work
	go func() {
		time.Sleep(m.Timeout)
		ack.timeout()
		delete(m.ReceivingNode.Acks, id)
	}()

	return ack
}

func (m *OutgoingMessage) isPrivateMessage() bool {
	helpers.Assert(m.ToNode != nil, "How the hell did we get here? This should have been caught earlier")
	return m.ToNode.Id != Broadcast.Id
}

func (m *OutgoingMessage) delivered(ack *acknowledgement) bool {
	if m.isPrivateMessage() {
		return <-ack.delivered
	} else {
		return <-ack.repeated
	}
}

func (m *OutgoingMessage) String() string {
	helpers.Assert(m.FromNode != nil, "I should have a known FromNode at this point")
	helpers.Assert(m.ToNode != nil, "I should have a known ToNode at this point")

	direction := m.FromNode.ColorString()
	if m.isPrivateMessage() {
		direction += " -> " + m.ToNode.ColorString()
	} else {
		direction += " -> Channel " + m.Channel.name
	}

	return fmt.Sprintf("%s:\n%s", direction, helpers.Indent(m.Text, "\t"))
}
