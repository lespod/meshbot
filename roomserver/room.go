package roomserver

import (
	"fmt"
	"strings"

	"github.com/timendus/meshbot/config"
	m "github.com/timendus/meshbot/meshwrapper"
)

type Room struct {
	Config   config.Room
	Messages []Message
	Users    []*User
}

type Message struct {
	Sender   *User
	Contents string
}

type User struct {
	Node    *m.Node
	Send    func(string) chan bool
	Backlog []*Message
}

var Rooms []Room
var Users map[*m.Node]*User

func Init(cfg config.Config) {
	for _, room := range cfg.Rooms {
		Rooms = append(Rooms, Room{
			Config: room,
		})
	}
	Users = make(map[*m.Node]*User)
}

func GetUser(msg m.Message) *User {
	if user, ok := Users[msg.FromNode]; ok {
		return user
	}
	user := &User{
		Node: msg.FromNode,
		Send: func(m string) chan bool { return msg.Reply(m) },
	}
	Users[msg.FromNode] = user
	return user
}

func RoomList(user *User) string {
	text := ""
	for _, room := range Rooms {
		public := " ✅ "
		if room.Config.Password != "" {
			public = " 🔐 "
		}
		joined := ""
		if room.present(user) {
			joined = " (joined)"
		}
		text += public + room.Config.Name + joined + "\n"
	}
	return text
}

func Join(user *User, roomName string, password string) error {
	roomName = strings.ToLower(roomName)
	for i, room := range Rooms {
		if roomName == strings.ToLower(room.Config.Name) {
			if room.Config.Password != "" && room.Config.Password != password {
				return fmt.Errorf("Invalid password for %s", room.Config.Name)
			}
			if room.present(user) {
				return fmt.Errorf("You are already in room %s", room.Config.Name)
			}
			Rooms[i].Users = append(Rooms[i].Users, user)
			return nil
		}
	}
	return fmt.Errorf("Can't find that room!")
}

func Leave(user *User, roomName string) error {
	roomName = strings.ToLower(roomName)
	for i, room := range Rooms {
		if roomName == strings.ToLower(room.Config.Name) {
			for j, u := range room.Users {
				if u.Node.Id == user.Node.Id {
					Rooms[i].Users = append(Rooms[i].Users[:j], Rooms[i].Users[j+1:]...)
					return nil
				}
			}
			return fmt.Errorf("Looks like you were not in room %s", roomName)
		}
	}
	return fmt.Errorf("Can't find that room!")
}

func RoomsForUser(user *User) []Room {
	rooms := make([]Room, 0)
	for _, room := range Rooms {
		if room.present(user) {
			rooms = append(rooms, room)
		}
	}
	return rooms
}

func Send(user *User, message string) error {
	rooms := RoomsForUser(user)
	switch len(rooms) {
	case 0:
		firstPublicRoom := ""
		for _, room := range Rooms {
			if room.Config.Password == "" {
				firstPublicRoom = room.Config.Name
			}
		}
		if firstPublicRoom == "" {
			return fmt.Errorf("You're not in any rooms. /join a room.")
		}
		err := Join(user, firstPublicRoom, "")
		if err != nil {
			return fmt.Errorf("You're not in any rooms. /join a room.")
		}
		return fmt.Errorf("You were not in any rooms. I took the liberty of putting you in room %s.\n\n🔴 Note: All messages you send to me from now on will be broadcast to room %s! 🔴", firstPublicRoom, firstPublicRoom)
	case 1:
		rooms[0].send(Message{Sender: user, Contents: message})
		return nil
	default:
		parts := strings.Split(message, " ")
		roomName := strings.ToLower(parts[0])
		for _, room := range rooms {
			if roomName == strings.ToLower(room.Config.Name) {
				room.send(Message{Sender: user, Contents: strings.Join(parts[1:], " ")})
				return nil
			}
		}
		return fmt.Errorf("You've joined multiple rooms, please prefix your message with the name of the room you want to send to.")
	}
}

func (room *Room) send(msg Message) {
	room.Messages = append(room.Messages, msg)
	for _, user := range room.Users {
		go func() {
			ok := <-user.Send("[" + msg.Sender.Node.ShortName + "] " + msg.Contents)
			if !ok {
				user.Backlog = append(user.Backlog, &msg)
			}
		}()
	}
}

func (room *Room) present(user *User) bool {
	for _, u := range room.Users {
		if u == user {
			return true
		}
	}
	return false
}
