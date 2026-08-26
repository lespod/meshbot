package meshwrapper

import (
	"log"
	"math/rand/v2"
	"sync/atomic"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"github.com/timendus/meshbot/config"
)

type acknowledgement struct {
	id        int32
	waiting   atomic.Bool
	delivered chan bool
	repeated  chan bool
	recipient *Node
	status    string
}

// Utwórz potwierdzenie dla wiadomości wysyłanej do wskazanego noda.
func newAcknowledgement(node *Node) *acknowledgement {
	ack := acknowledgement{
		id:        rand.Int32(),
		delivered: make(chan bool, 1),
		repeated:  make(chan bool, 1),
		recipient: node,
		waiting:   atomic.Bool{},
		status:    "Oczekiwanie",
	}
	ack.waiting.Store(true)
	ack.spam()
	return &ack
}

func (a *acknowledgement) receive(node *Node, err meshtastic.Routing_Error) {
	if !a.waiting.Load() {
		if config.IsLogEnabled("acknowledgements") {
			log.Printf("Potwierdzenie %d do %s: odebrano pakiet, ale już nie oczekiwano\n", a.id, a.recipient.ColorString())
		}
		return
	}
	if err != meshtastic.Routing_NONE {
		a.negative("Błąd routingu: " + meshtastic.Routing_Error_name[int32(err)])
		return
	}
	if node.Id == a.recipient.Id {
		a.status = "Doręczono"
		a.delivered <- true
		a.repeated <- false
		a.close()
	} else {
		a.status = "Powtórzono"
		a.repeated <- true
		a.spam()
	}
}

func (a *acknowledgement) timeout() {
	a.negative("Przekroczono czas oczekiwania")
}

func (a *acknowledgement) error(err error) {
	a.negative("Nie udało się wysłać wiadomości: " + err.Error())
}

func (a *acknowledgement) negative(reason string) {
	if !a.waiting.Load() {
		return
	}
	a.status = reason
	a.delivered <- false
	a.repeated <- false
	a.close()
}

func (a *acknowledgement) close() {
	a.waiting.Store(false)
	close(a.delivered)
	close(a.repeated)
	a.spam()
}

func (a *acknowledgement) spam() {
	if config.IsLogEnabled("acknowledgements") {
		log.Printf("Potwierdzenie %d do %s: %s\n", a.id, a.recipient.ColorString(), a.status)
	}
}
