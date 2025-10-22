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
	Node     *m.Node
	Send     func(string) chan bool
	Backlog  []*Message
	Selected *Room
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
	for i, room := range Rooms {
		public := " ✅ "
		if room.Config.Password != "" {
			public = " 🔐 "
		}
		joined := ""
		if room.present(user) {
			joined = " (joined)"
		}
		selected := ""
		if user.Selected == &Rooms[i] {
			selected = " (selected)"
		}
		text += public + room.Config.Name + joined + selected + "\n"
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
			user.Selected = &Rooms[i]
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
					user.autoSelectRoom()
					return nil
				}
			}
			return fmt.Errorf("Looks like you were not in room %s", roomName)
		}
	}
	return fmt.Errorf("Can't find that room!")
}

func Select(user *User, roomName string) error {
	roomName = strings.ToLower(roomName)
	for i, room := range Rooms {
		if roomName == strings.ToLower(room.Config.Name) {
			return user.selectRoom(&Rooms[i])
		}
	}
	return fmt.Errorf("Can't find that room!")
}

func Send(user *User, message string) error {
	rooms := user.rooms()

	if len(rooms) == 0 {
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
	}

	if user.Selected == nil {
		return fmt.Errorf("You have not selected a room to send to. Please /select a room.")
	}
	user.Selected.send(Message{Sender: user, Contents: message})
	return nil
}

func (room *Room) send(msg Message) {
	room.Messages = append(room.Messages, msg)
	for _, user := range room.Users {
		go func() {
			ok := <-user.Send("[" + msg.Sender.Node.ShortName + " in " + room.Config.Name + "] " + msg.Contents)
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

func (user *User) rooms() []Room {
	rooms := make([]Room, 0)
	for _, room := range Rooms {
		if room.present(user) {
			rooms = append(rooms, room)
		}
	}
	return rooms
}

func (user *User) selectRoom(room *Room) error {
	if !room.present(user) {
		return fmt.Errorf("You can't select a room you haven't joined")
	}

	if user.Selected == room {
		return fmt.Errorf("Room %s was already selected", room.Config.Name)
	}

	user.Selected = room
	return nil
}

func (user *User) autoSelectRoom() {
	rooms := user.rooms()

	// If no rooms; no selection
	if len(rooms) == 0 {
		user.Selected = nil
		return
	}

	// If selected is a valid room, keep it
	if user.Selected != nil {
		for i := range rooms {
			if user.Selected == &rooms[i] {
				return
			}
		}
	}

	// Otherwise, select the first room
	user.Selected = &rooms[0]
}
