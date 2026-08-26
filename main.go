/*
Copyright (C) 2026 Timendus
Modifications and extensions Copyright (C) 2026 lespod

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://gnu.org>.
*/

package main

// https://meshtastic.org/docs/development/device/client-api/
// https://buf.build/meshtastic/protobufs/docs/main:meshtastic#meshtastic.ToRadio

import (
	"fmt"
	"io"
	"log"
	"maps"
	"math"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/timendus/meshbot/config"
	m "github.com/timendus/meshbot/meshwrapper"
	"github.com/timendus/meshbot/meshwrapper/helpers"
	"github.com/timendus/meshbot/weather"
	"go.bug.st/serial"
)

func main() {
	log.Println("Uruchamiam Meshed Potatoes!")
	err := config.InitConfig()
	if err != nil {
		log.Println("Wystąpił problem podczas odczytu config.json:")
		log.Fatal(err)
	}
	cfg := config.GetConfig()

	m.IncomingMessageEvents.Subscribe(m.IncomingMessageEvent, incoming)
	m.IncomingMessageEvents.Subscribe(m.TraceRouteEvent, traceroute)
	m.OutgoingMessageEvents.Subscribe(m.OutgoingMessageEvent, outgoing)
	m.ConnectionEvents.Subscribe(m.ConnectedEvent, connected)
	m.ConnectionEvents.Subscribe(m.DisconnectedEvent, disconnected)

	// Połącz z urządzeniami Meshtastic wymienionymi w konfiguracji.
	for _, connection := range cfg.Connections {
		var node *m.ConnectedNode
		var err error

		switch connection.ConnectionType {
		case config.SERIAL_CONNECTION:
			if !cfg.Settings.AllowSerial {
				log.Fatal("Połączenie serial jest skonfigurowane, ale ustawienia na nie nie pozwalają")
			}
			node = m.NewConnectedNode(func() (io.ReadWriteCloser, error) {
				if config.IsLogEnabled("connections") {
					log.Println("Próbuję otworzyć połączenie serial z " + connection.SerialDevice)
				}
				stream, err := serial.Open(connection.SerialDevice, &serial.Mode{
					BaudRate: 115200,
				})
				if err != nil {
					return nil, fmt.Errorf("nie udało się otworzyć połączenia serial z '"+connection.SerialDevice+"': %w", err)
				}
				return stream, nil
			})

		case config.TCP_CONNECTION:
			if !cfg.Settings.AllowTCP {
				log.Fatal("Połączenie TCP jest skonfigurowane, ale ustawienia na nie nie pozwalają")
			}
			node = m.NewConnectedNode(func() (io.ReadWriteCloser, error) {
				conn := connection.Hostname + ":" + strconv.Itoa(connection.Port)
				if config.IsLogEnabled("connections") {
					log.Println("Próbuję otworzyć połączenie TCP z " + conn)
				}
				stream, err := net.Dial("tcp", conn)
				if err != nil {
					return nil, fmt.Errorf("nie udało się otworzyć połączenia TCP z '"+conn+"': %w", err)
				}
				return stream, nil
			})

		default:
			log.Fatal("Nieprawidłowy typ połączenia!")
		}

		err = node.Connect()
		if err != nil {
			log.Fatal(err)
		}
		defer node.Close()
	}

	// Pętla utrzymująca proces przy życiu.
	for {
		time.Sleep(100 * time.Millisecond)
	}
}

// Do późniejszego użycia.
func getSerialDevices() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return ports, err
	}

	if len(ports) > 0 {
		log.Printf("Znaleziono %d portów serial:\n", len(ports))
		for i, port := range ports {
			log.Printf("  [%d] %s\n", i, port)
		}
	}

	return ports, err
}

var announcersRunning bool

type pendingTraceroute struct {
	command m.IncomingMessage
	target  *m.Node
	done    chan struct{}
}

var pendingTraceroutes = struct {
	sync.Mutex
	requests map[uint32]*pendingTraceroute
}{
	requests: make(map[uint32]*pendingTraceroute),
}

func connected(node m.ConnectedNode) {
	if config.IsLogEnabled("connections") {
		log.Println("Połączono z " + node.String())
	}
	// log.Println("Lista nodów: \n" + node.NodeList.String())

	// Pokaż dostępne kanały w logach.
	if config.IsLogEnabled("channels") {
		log.Println("Lista kanałów:")
		keys := slices.Collect(maps.Keys(node.Channels))
		slices.Sort(keys)
		for _, key := range keys {
			channel := node.Channels[key]
			log.Println("   " + channel.String())
		}
	}

	// Uruchom zaplanowane ogłoszenia.
	if !announcersRunning {
		for _, announcement := range config.GetConfig().Announcements {
			channel, ok := node.FindChannel(announcement.Channel)
			if !ok {
				log.Printf("Ogłoszenia: nie mogę znaleźć kanału %s\n", announcement.Channel)
				continue
			}
			go func() {
				for {
					if config.IsLogEnabled("announcements") {
						log.Println("Czas na ogłoszenie!")
					}
					m.NewOutgoingChannelMessage(announcement.Message, &node, channel, announcement.MaxHops).Send()
					time.Sleep(time.Duration(announcement.DelayMinutes) * time.Minute)
				}
			}()
		}
		announcersRunning = true
	}
}

func disconnected(node m.ConnectedNode) {
	if config.IsLogEnabled("connections") {
		log.Println("Rozłączono z nodem")
	}
	backoff := 1 * time.Second
	for !node.Connected {
		if config.IsLogEnabled("connections") {
			log.Println("Próbuję ponownie połączyć się z nodem...")
		}
		err := node.Connect()
		if err == nil {
			if config.IsLogEnabled("connections") {
				log.Println("Ponownie połączono z nodem!")
			}
			break
		}
		if config.IsLogEnabled("connections") {
			log.Println("Nie udało się połączyć, wydłużam czas oczekiwania...", err)
		}
		time.Sleep(backoff)
		backoff *= 2
	}
}

func commandMatches(command string, aliases ...string) (string, bool) {
	trimmed := strings.TrimSpace(command)
	upper := strings.ToUpper(trimmed)
	for _, alias := range aliases {
		alias = strings.ToUpper(alias)
		if upper == alias {
			return "", true
		}
		if strings.HasPrefix(upper, alias+" ") {
			return strings.TrimSpace(trimmed[len(alias):]), true
		}
	}
	return "", false
}

func enabledCommandMatches(command string, name string, aliases ...string) (string, bool) {
	args, ok := commandMatches(command, aliases...)
	return args, ok && config.IsCommandEnabled(name)
}

func disabledCommandMatches(command string) bool {
	commands := []struct {
		name    string
		aliases []string
	}{
		{"ping", []string{"/PING"}},
		{"pomoc", []string{"/POMOC", "/HELP", "/O", "/ABOUT"}},
		{"sygnal", []string{"/SYGNAL", "/SYGNAŁ", "/SIGNAL"}},
		{"sasiedzi", []string{"/SASIEDZI", "/SĄSIEDZI", "/NEIGHBOURS", "/NEIGHBORS"}},
		{"hopy", []string{"/HOPY", "/HOPS"}},
		{"test", []string{"/TEST", "TEST"}},
		{"sciezka", []string{"/SCIEZKA", "/ŚCIEŻKA", "/TRASA", "/TRACE", "/TRACEROUTE"}},
		{"pogoda", []string{"/POGODA", "/WEATHER"}},
		{"prognoza", []string{"/PROGNOZA", "/FORECAST"}},
	}
	for _, commandConfig := range commands {
		if _, ok := commandMatches(command, commandConfig.aliases...); ok {
			return !config.IsCommandEnabled(commandConfig.name)
		}
	}
	return false
}

func botPosition(message m.IncomingMessage) [3]float32 {
	pos := message.ReceivingNode.Node.GetPosition()
	if pos[0] == 0 && pos[1] == 0 && message.ToNode != nil {
		pos = message.ToNode.GetPosition()
	}
	return pos
}

func localityText(pos [3]float32) string {
	if pos[0] == 0 && pos[1] == 0 {
		return "lokalizacja nieznana"
	}
	locality, err := weather.FetchLocality(weather.Position{
		Latitude:  float64(pos[0]),
		Longitude: float64(pos[1]),
	})
	if err != nil || locality == "" {
		return "lokalizacja nieznana"
	}
	return locality
}

func distanceMeters(from [3]float32, to [3]float32) (float64, bool) {
	if (from[0] == 0 && from[1] == 0) || (to[0] == 0 && to[1] == 0) {
		return 0, false
	}
	const earthRadiusMeters = 6371000
	lat1 := float64(from[0]) * math.Pi / 180
	lat2 := float64(to[0]) * math.Pi / 180
	dLat := (float64(to[0]) - float64(from[0])) * math.Pi / 180
	dLon := (float64(to[1]) - float64(from[1])) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c, true
}

func formatDistance(meters float64) string {
	if meters < 1000 {
		return fmt.Sprintf("%.0f m", meters)
	}
	if meters < 10000 {
		return fmt.Sprintf("%.1f km", meters/1000)
	}
	return fmt.Sprintf("%.0f km", meters/1000)
}

func compactNodeName(node *m.Node, id uint32) string {
	if node == nil {
		return fmt.Sprintf("!%04x", id&0xffff)
	}
	if node.ShortName != "" && node.ShortName != "UNKN" {
		return node.ShortName
	}
	return fmt.Sprintf("!%04x", id&0xffff)
}

func routeWithEndpoints(route []uint32, start uint32, end uint32) []uint32 {
	result := make([]uint32, 0, len(route)+2)
	if start != 0 {
		result = append(result, start)
	}
	for _, id := range route {
		if id == 0 {
			continue
		}
		if len(result) == 0 || result[len(result)-1] != id {
			result = append(result, id)
		}
	}
	if end != 0 && (len(result) == 0 || result[len(result)-1] != end) {
		result = append(result, end)
	}
	return result
}

func formatRoute(node *m.ConnectedNode, ids []uint32) string {
	if len(ids) == 0 {
		return "?"
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, compactNodeName(node.FindNodeById(id), id))
	}
	return strings.Join(parts, ">")
}

func formatTraceroute(message m.IncomingMessage, request *pendingTraceroute) string {
	if message.RouteDiscovery == nil {
		return "🤖🧭 brak trasy"
	}

	botID := request.command.ReceivingNode.Node.Id
	targetID := request.target.Id
	towards := routeWithEndpoints(message.RouteDiscovery.GetRoute(), botID, targetID)
	back := routeWithEndpoints(message.RouteDiscovery.GetRouteBack(), targetID, botID)

	return "🤖🧭 tam: " + formatRoute(message.ReceivingNode, towards) + "\nwr: " + formatRoute(message.ReceivingNode, back)
}

func rememberTraceroute(requestID uint32, request *pendingTraceroute) {
	pendingTraceroutes.Lock()
	pendingTraceroutes.requests[requestID] = request
	pendingTraceroutes.Unlock()

	go func() {
		select {
		case <-request.done:
			return
		case <-time.After(60 * time.Second):
		}

		pendingTraceroutes.Lock()
		_, pending := pendingTraceroutes.requests[requestID]
		if pending {
			delete(pendingTraceroutes.requests, requestID)
		}
		pendingTraceroutes.Unlock()

		if pending {
			request.command.ReplyTo("🤖🧭 timeout")
		}
	}()
}

func takeTraceroute(requestID uint32, fromID uint32) *pendingTraceroute {
	pendingTraceroutes.Lock()
	defer pendingTraceroutes.Unlock()

	if request, ok := pendingTraceroutes.requests[requestID]; ok {
		delete(pendingTraceroutes.requests, requestID)
		close(request.done)
		return request
	}

	if requestID != 0 {
		return nil
	}
	for id, request := range pendingTraceroutes.requests {
		if request.target != nil && request.target.Id == fromID {
			delete(pendingTraceroutes.requests, id)
			close(request.done)
			return request
		}
	}
	return nil
}

func incoming(message m.IncomingMessage) {
	if config.IsLogEnabled(incomingLogCategory(message)) {
		fmt.Println(message.String())
	}

	if message.MessageType != m.MESSAGE_TYPE_TEXT_MESSAGE {
		return
	}

	command := strings.TrimSpace(message.Text)
	if disabledCommandMatches(command) {
		return
	}

	if _, ok := enabledCommandMatches(command, "ping", "/PING"); ok {
		message.ReplyTo("🤖🏓 Pong!")
		return
	}

	if _, ok := enabledCommandMatches(command, "pomoc", "/POMOC", "/HELP", "/O", "/ABOUT"); ok {
		message.Reply(
			`🤖 /ping /sygnal [node] /sasiedzi /hopy /test /pogoda /prognoza
DM: /sciezka`)
		return
	}

	if needle, ok := enabledCommandMatches(command, "sygnal", "/SYGNAL", "/SYGNAŁ", "/SIGNAL"); ok {
		subject := message.FromNode
		if needle != "" {
			subject = message.FindNode(needle)
		}

		if subject == nil {
			message.Reply("🤖🧨 Nie znam noda.")
			return
		}

		if subject.HopsAway == 0 {
			message.Reply("🤖📶 " + subject.String() + " " + helpers.TimeAgo(subject.LastHeard) + " temu, SNR " +
				strconv.FormatFloat(float64(subject.GetSNR()), 'f', 2, 32))
		} else {
			message.Reply("🤖📶 " + subject.String() + ": " + strconv.Itoa(int(subject.HopsAway)) + " " + helpers.PolishHopWord(int(subject.HopsAway)))
		}
		return
	}

	if _, ok := enabledCommandMatches(command, "sasiedzi", "/SASIEDZI", "/SĄSIEDZI", "/NEIGHBOURS", "/NEIGHBORS"); ok {
		message.Reply("🤖👂 1h:\n" + message.ReceivingNode.NodeList.Neighbours())
		return
	}

	if _, ok := enabledCommandMatches(command, "hopy", "/HOPY", "/HOPS"); ok {
		hops := int(message.HopsAway)
		locationText := localityText(botPosition(message))

		message.Reply("🤖📍 " + locationText + ", " + strconv.Itoa(hops) + " " + helpers.PolishJumpWord(hops))
		return
	}

	if _, ok := enabledCommandMatches(command, "test", "/TEST", "TEST"); ok {
		hops := int(message.HopsAway)
		botPos := botPosition(message)
		fromPos := [3]float32{}
		if message.FromNode != nil {
			fromPos = message.FromNode.GetPosition()
		}
		distanceText := "dystans nieznany"
		if meters, ok := distanceMeters(fromPos, botPos); ok {
			distanceText = "dystans: " + formatDistance(meters)
		}

		message.ReplyTo("🤖🧪 " + localityText(botPos) + ", " + strconv.Itoa(hops) + " " + helpers.PolishJumpWord(hops) + ", " + distanceText)
		return
	}

	if _, ok := enabledCommandMatches(command, "sciezka", "/SCIEZKA", "/ŚCIEŻKA", "/TRASA", "/TRACE", "/TRACEROUTE"); ok {
		if !message.IsPrivateMessage() {
			return
		}
		if message.FromNode == nil {
			message.ReplyTo("🤖🧭 brak noda")
			return
		}
		requestID, err := message.ReceivingNode.SendTraceroute(message.FromNode, uint32(min(int(message.HopsAway)+2, 7)))
		if err != nil {
			log.Println("Nie udało się wysłać traceroute:", err)
			message.ReplyTo("🤖🧭 błąd")
			return
		}
		rememberTraceroute(requestID, &pendingTraceroute{
			command: message,
			target:  message.FromNode,
			done:    make(chan struct{}),
		})
		return
	}

	if _, ok := enabledCommandMatches(command, "pogoda", "/POGODA", "/WEATHER"); ok {
		var text string
		var pos [3]float32
		if message.FromNode != nil {
			pos = message.FromNode.GetPosition()
			text = "u Ciebie:"
		}
		if message.FromNode == nil || pos[0] == 0 || pos[1] == 0 {
			pos = message.ToNode.GetPosition()
			text = "u mnie:"
		}
		if pos[0] == 0 || pos[1] == 0 {
			message.Reply("🤖🧨 Brak lok.")
			return
		}
		weather, err := weather.FetchWeather(weather.Position{
			Latitude:  float64(pos[0]),
			Longitude: float64(pos[1]),
		})
		if err != nil {
			message.Reply("🤖🌂 Błąd pogody.")
		} else {
			ok := <-message.Reply("🤖🌂 " + text + "\n\n" + weather)
			if !ok {
				log.Println("Nie udało się wysłać pełnej wiadomości pogodowej :/")
			}
		}
		return
	}

	if _, ok := enabledCommandMatches(command, "prognoza", "/PROGNOZA", "/FORECAST"); ok {
		var text string
		var pos [3]float32
		if message.FromNode != nil {
			pos = message.FromNode.GetPosition()
			text = "u Ciebie:"
		}
		if message.FromNode == nil || pos[0] == 0 || pos[1] == 0 {
			pos = message.ToNode.GetPosition()
			text = "u mnie:"
		}
		if pos[0] == 0 || pos[1] == 0 {
			message.Reply("🤖🧨 Brak lok.")
			return
		}
		forecast, err := weather.FetchForecast(weather.Position{
			Latitude:  float64(pos[0]),
			Longitude: float64(pos[1]),
		})
		if err != nil {
			message.Reply("🤖🌂 Błąd prognozy.")
		} else {
			ok := <-message.Reply("🤖🌂 " + text + "\n\n" + forecast)
			if !ok {
				log.Println("Nie udało się wysłać pełnej wiadomości pogodowej :/")
			}
		}
		return
	}

	// Zwykłe wiadomości oraz nieznane komendy nie są przez bota obsługiwane.
	if message.IsPrivateMessage() && strings.HasPrefix(command, "/") {
		message.ReplyReliably("🤖❓ ? /pomoc")
	}
}

func incomingLogCategory(message m.IncomingMessage) string {
	switch message.MessageType {
	case m.MESSAGE_TYPE_TEXT_MESSAGE:
		return "incoming_messages"
	case m.MESSAGE_TYPE_TELEMETRY_DEVICE,
		m.MESSAGE_TYPE_TELEMETRY_ENVIRONMENT,
		m.MESSAGE_TYPE_TELEMETRY_HEALTH,
		m.MESSAGE_TYPE_TELEMETRY_AIR_QUALITY,
		m.MESSAGE_TYPE_TELEMETRY_POWER,
		m.MESSAGE_TYPE_TELEMETRY_LOCAL_STATS:
		return "incoming_telemetry"
	default:
		return "incoming_packets"
	}
}

func traceroute(message m.IncomingMessage) {
	if message.FromNode == nil {
		return
	}
	request := takeTraceroute(message.RequestId, message.FromNode.Id)
	if request == nil {
		return
	}
	request.command.ReplyTo(formatTraceroute(message, request))
}

func outgoing(message m.OutgoingMessage) {
	if config.IsLogEnabled("outgoing_messages") {
		fmt.Println(message.String())
	}
}
