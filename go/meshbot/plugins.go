package meshbot

import (
	"context"
	"errors"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type plugin struct {
	Name        string
	Description string
	Version     string
	Hidden      bool
	Commands    []command
	States      []State
	LuaState    *lua.LState
}

type command struct {
	State          State
	Command        []string
	Prefix         []string
	Description    string
	Private        bool
	Channel        bool
	IsCatchAll     bool
	IsCatchAllText bool
	Hidden         bool
	Function       func(*ChatMessage) (State, error)
}

type contextKey string

const (
	luaMessageTypeName = "message"
	CATCH_ALL_EVENTS   = iota
	CATCH_ALL_TEXT
)

func LoadPlugin(filename string) (*plugin, error) {
	L := createLuaVM()
	if err := L.DoFile(filename); err != nil {
		return nil, err
	}
	definition, ok := L.GetGlobal("plugin").(*lua.LTable)
	if !ok {
		return nil, errors.New("no plugin definition found in file " + filename)
	}
	return newPlugin(definition, L), nil
}

func newPlugin(definition *lua.LTable, L *lua.LState) *plugin {
	plugin := plugin{
		Name:        definition.RawGetString("name").String(),
		Description: definition.RawGetString("description").String(),
		Version:     definition.RawGetString("version").String(),
		Hidden:      lua.LVAsBool(definition.RawGetString("hidden")),
		Commands:    make([]command, 0),
		States:      make([]State, 0),
		LuaState:    L,
	}

	commands := definition.RawGetString("commands")
	if commands, ok := commands.(*lua.LTable); ok {
		commands.ForEach(func(k, v lua.LValue) {
			command := newCommand(v.(*lua.LTable), L)
			plugin.Commands = append(plugin.Commands, command)
		})
	}

	states := definition.RawGetString("states")
	if states, ok := states.(*lua.LTable); ok {
		states.ForEach(func(k, v lua.LValue) {
			plugin.States = append(plugin.States, State(v.String()))
		})
	}

	return &plugin
}

func newCommand(definition *lua.LTable, L *lua.LState) command {
	state := definition.RawGetString("state").String()
	if state == "nil" {
		state = "MAIN"
	}

	private := definition.RawGetString("private")
	if private == lua.LNil {
		private = lua.LTrue
	}

	command := command{
		State:          State(state),
		Command:        make([]string, 0),
		Prefix:         make([]string, 0),
		Description:    definition.RawGetString("description").String(),
		Private:        lua.LVAsBool(private),
		Channel:        lua.LVAsBool(definition.RawGetString("channel")),
		IsCatchAll:     false,
		IsCatchAllText: false,
		Hidden:         lua.LVAsBool(definition.RawGetString("hidden")),
		Function: func(message *ChatMessage) (State, error) {
			function, ok := definition.RawGetString("func").(*lua.LFunction)
			if !ok {
				return "ERROR", nil
			}
			messageUserData := L.NewUserData()
			messageUserData.Value = message
			L.SetMetatable(messageUserData, L.GetTypeMetatable(luaMessageTypeName))
			err := L.CallByParam(lua.P{
				Fn:      function,
				NRet:    1,
				Protect: true,
			}, messageUserData)
			if err != nil {
				return "ERROR", err
			}
			ret := L.Get(-1)
			L.Pop(1)
			if ret.Type() == lua.LTNil {
				return "MAIN", nil
			} else {
				return State(ret.String()), nil
			}
		},
	}

	commands := definition.RawGetString("command")
	if commands, ok := commands.(*lua.LTable); ok {
		commands.ForEach(func(k, v lua.LValue) {
			command.Command = append(command.Command, v.String())
		})
	}
	if cmd, ok := commands.(lua.LString); ok {
		command.Command = append(command.Command, cmd.String())
	}

	prefixes := definition.RawGetString("prefix")
	if prefixes, ok := prefixes.(*lua.LTable); ok {
		prefixes.ForEach(func(k, v lua.LValue) {
			command.Prefix = append(command.Prefix, v.String())
		})
	}
	if prefix, ok := prefixes.(lua.LString); ok {
		command.Prefix = append(command.Prefix, prefix.String())
	}

	if cmd, ok := commands.(lua.LNumber); ok {
		if cmd == CATCH_ALL_EVENTS {
			command.IsCatchAll = true
		}
		if cmd == CATCH_ALL_TEXT {
			command.IsCatchAllText = true
		}
	}

	return command
}

func createLuaVM() *lua.LState {
	// Initialize a bare-bones Lua VM
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	lua.OpenBase(L)

	// Make some properties of the bot available to Lua
	bot := L.NewTable()
	L.SetGlobal("bot", bot)
	bot.RawSetString("CATCH_ALL_TEXT", lua.LNumber(CATCH_ALL_TEXT))
	bot.RawSetString("CATCH_ALL_EVENTS", lua.LNumber(CATCH_ALL_EVENTS))
	botMT := L.NewTable()
	botMT.RawSetString("__tostring", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString("Hello, world!"))
		return 1
	}))
	L.SetMetatable(bot, botMT)

	// This is pretty crude, but it provides a way to save some data from the
	// Lua scripts, that we can actually persist and make thread safe in the
	// future.
	L.SetContext(context.WithValue(context.Background(), contextKey("storage"), make(map[string]string)))
	memory := L.NewTable()
	memory.RawSetString("write", L.NewFunction(func(L *lua.LState) int {
		ctx := L.Context()
		key := L.CheckString(1)
		ctx.Value(contextKey("storage")).(map[string]lua.LValue)[key] = L.Get(2)
		return 0
	}))
	memory.RawSetString("read", L.NewFunction(func(L *lua.LState) int {
		ctx := L.Context()
		key := L.CheckString(1)
		value := ctx.Value(contextKey("storage")).(map[string]lua.LValue)[key]
		L.Push(value)
		return 1
	}))
	bot.RawSetString("memory", memory)

	// Register the Message usertype
	mt := L.NewTypeMetatable(luaMessageTypeName)
	L.SetGlobal(luaMessageTypeName, mt)
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), messageMethods))

	return L
}

var messageMethods = map[string]lua.LGFunction{
	"reply":         messageReply,
	"replyBlocking": messageReplyBlocking,
	"text":          messageText,
}

// Checks whether the first lua argument is a *LUserData with *ChatMessage and returns this *ChatMessage
func checkMessage(L *lua.LState) *ChatMessage {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*ChatMessage); ok {
		return v
	}
	L.ArgError(1, "message expected")
	return nil
}

func messageReply(L *lua.LState) int {
	message := *checkMessage(L)
	message.Reply(L.CheckString(2))
	return 0
}

func messageReplyBlocking(L *lua.LState) int {
	message := *checkMessage(L)
	timeout := time.Second * time.Duration(L.OptInt(3, int(DEFAULT_BLOCKING_MESSAGE_TIMEOUT)))
	delivered := <-message.ReplyBlocking(L.CheckString(2), timeout)
	L.Push(lua.LBool(delivered))
	return 1
}

func messageText(L *lua.LState) int {
	message := *checkMessage(L)
	L.Push(lua.LString(message.GetText()))
	return 1
}
