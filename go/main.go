package main

// https://meshtastic.org/docs/development/device/client-api/
// https://buf.build/meshtastic/protobufs/docs/main:meshtastic#meshtastic.ToRadio

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/timendus/meshbot/config"
	"github.com/timendus/meshbot/meshbot"
	m "github.com/timendus/meshbot/meshwrapper"
	"go.bug.st/serial"
)

var bot *meshbot.Chatbot

func main() {
	log.Println("Starting Meshed Potatoes!")
	config.InitConfig()
	cfg := config.GetConfig()

	m.MessageEvents.Subscribe(m.AnyMessageEvent, message)
	m.ConnectionEvents.Subscribe(m.ConnectedEvent, connected)
	m.ConnectionEvents.Subscribe(m.DisconnectedEvent, disconnected)

	// Connect to the meshtastic devices mentioned in the configuration file
	for _, connection := range cfg.Connections {
		var node *m.ConnectedNode
		var port io.ReadWriteCloser
		var err error

		switch connection.ConnectionType {
		case config.SERIAL_CONNECTION:
			if !cfg.Settings.AllowSerial {
				log.Fatal("Serial connection configured, but not allowed by settings")
			}
			port, err = serial.Open(connection.SerialDevice, &serial.Mode{
				BaudRate: 115200,
			})
			if err != nil {
				log.Fatal("Could not open serial connection to '"+connection.SerialDevice+"': ", err)
			}
		case config.TCP_CONNECTION:
			if !cfg.Settings.AllowTCP {
				log.Fatal("TCP connection configured, but not allowed by settings")
			}
			port, err = net.Dial("tcp", connection.Hostname+":"+strconv.Itoa(connection.Port))
			if err != nil {
				log.Fatal("Could not open TCP connection to '"+connection.Hostname+":"+strconv.Itoa(connection.Port)+"': ", err)
			}

		default:
			log.Fatal("Invalid connection type!")
		}

		node, err = m.NewConnectedNode(port)
		if err != nil {
			log.Fatal(err)
		}
		defer node.Close()
	}

	// Launch the chat bot
	bot = meshbot.NewChatbot()
	err := bot.ReloadPlugins()
	if err != nil {
		log.Fatal(err)
	}

	// Endless loop to keep the program from ending
	for {
		time.Sleep(100 * time.Millisecond)
	}
}

// For later use
func getSerialDevices() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return ports, err
	}

	if len(ports) > 0 {
		log.Printf("Found %d serial ports:\n", len(ports))
		for i, port := range ports {
			log.Printf("  [%d] %s\n", i, port)
		}
	}

	return ports, err
}

func connected(node m.ConnectedNode) {
	log.Println("Connected to " + node.String())
	// log.Println("Node list: \n" + node.NodeList.String())
	// log.Println("Channel list:")
	// for _, channel := range node.Channels {
	// 	log.Println("   " + channel.String())
	// }
}

func disconnected(node m.ConnectedNode) {
	log.Println("Disconnected from the node. Maybe some retry-logic here?")
}

func message(message m.Message) {
	fmt.Println(message.String())
	if bot != nil {
		bot.HandleMessage(message)
	}
}
