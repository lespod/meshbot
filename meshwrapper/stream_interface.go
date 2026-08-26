package meshwrapper

import (
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"github.com/timendus/meshbot/config"
	"google.golang.org/protobuf/proto"
)

const (
	START1   = 0x94
	START2   = 0xC3
	MAX_SIZE = 512
)

func wakeDevice(writer io.Writer) error {
	// Komentarze bazują na implementacji w Pythonie:
	// https://github.com/meshtastic/python/blob/0bb4b31b6a147134c57fb720492c8719c037d195/meshtastic/stream_interface.py#L55-L75

	// Wyślij puste znaki UART, żeby wybudzić urządzenie i zsynchronizować parser pakietów.
	bytes := make([]byte, 32)
	_, err := writer.Write(bytes)
	if err != nil {
		return err
	}

	// Poczekaj 100 ms, żeby urządzenie zdążyło ruszyć.
	time.Sleep(100 * time.Millisecond)
	return nil
}

func writeMessage(writer io.Writer, message *meshtastic.ToRadio) error {
	if config.IsLogEnabled("protocol_packets") {
		log.Println("\033[90mWysyłam: " + message.String() + "\033[0m")
	}

	bytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	header := [4]byte{START1, START2, byte(len(bytes) >> 8), byte(len(bytes) & 0xFF)}
	_, err = writer.Write(header[:])
	if err != nil {
		return err
	}

	_, err = writer.Write(bytes)
	if err != nil {
		return err
	}
	return nil
}

func readMessage(reader io.Reader) (*meshtastic.FromRadio, error) {
	buffer := make([]byte, 1)
	state := 0
	length := 0

searching:
	for {
		n, err := reader.Read(buffer)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, errors.New("nieoczekiwany koniec pliku")
		}

		switch state {
		case 0:
			if buffer[0] == START1 {
				state = 1
			} else if config.IsLogEnabled("protocol_packets") {
				// Pozostałe bajty potraktuj jako debug tekstowy.
				fmt.Print(buffer)
			}
		case 1:
			if buffer[0] == START2 {
				state = 2
			} else {
				state = 0
				if config.IsLogEnabled("protocol_packets") {
					fmt.Print([]byte{START1})
					fmt.Print(buffer)
				}
			}
		case 2:
			length = int(buffer[0]) << 8
			state = 3
		case 3:
			length |= int(buffer[0]) & 0xFF
			if length > MAX_SIZE {
				log.Printf("Nieprawidłowy rozmiar pakietu: %d\n", length)
				if config.IsLogEnabled("protocol_packets") {
					fmt.Print([]byte{START1, START2, byte(length >> 8)})
					fmt.Print(buffer)
				}
				state = 0
			} else if length == 0 {
				state = 0
			} else {
				break searching
			}
		}
	}

	protobuffer := make([]byte, length)
	n, err := io.ReadFull(reader, protobuffer)
	if err != nil {
		return nil, err
	}
	if n != length {
		return nil, errors.New("nieoczekiwany koniec pliku")
	}

	result := meshtastic.FromRadio{}
	err = proto.Unmarshal(protobuffer, &result)
	if err != nil {
		return nil, err
	}
	if config.IsLogEnabled("protocol_packets") {
		log.Println("\033[90mOdebrano: " + result.String() + "\033[0m")
	}
	return &result, nil
}
