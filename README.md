> This version of Meshbot is a rewrite in Go. If for some weird reason you
> **really** need the broken old Python version, I have saved that in the
> [branch
> legacy-version-in-python](https://github.com/Timendus/meshbot/tree/legacy-version-in-python).

# Meshbot

A simple bot for use with Meshtastic. I know the name isn't very original 😄

Some people would probably call this a "BBS", but personally I think it has more
in common with a crossover between a Slack / Telegram / Discord bot and a
MeshCore room server.

## Current features

- "Room server" that supports multiple rooms with subscriptions and
  semi-reliable message delivery ([see
  below](#why-rooms-are-more-reliable-than-channels) how we do that)
- Signal reports and neighbours can be queried
- Weather reports and forecasts can be queried, using
  [open-meteo.com](https://open-meteo.com/) (requires the bot to have an
  Internet connection)
- Programmable regular announcements to channels for service messages in your
  area

## Usage on the mesh

As a user of Meshbot, you can send these commands and Meshbot will reply.
Commands are not case-sensitive.

### Either in a channel or as a direct message

- `/about` or `/help` - Get a short overview of these commands
- `/signal <optional node>` - Fetch a signal report on yourself (default) or the
  node you ask for
- `/neighbours` - Fetch the list of neighbours that the bot can see over LoRa
- `/weather` - Fetch a report of the current weather conditions
- `/forecast` - Fetch a weather forecast for the coming days

### As direct messages only

- `/rooms` - Fetch a list of available rooms and your status in them
- `/join <room name> <optional password>` - Join a room, so you will receive
  messages sent to it. Supply a password for private rooms. Joining a room also
  selects it
- `/select <room name>` - Select a room. Messages you send will be broadcast to
  the selected room. Only a single room can be selected at a time. You can only
  select a room that you have joined
- `/leave <room name>` - Leave a room, so you will no longer receive messages
  sent to it

For any other DM you send to the bot:

- If you have not joined any rooms, and a public room exists (one without a
  password), it will make you join this room and select it automatically.
- Otherwise, any DM you send to the bot will be sent to all users in the
  selected room.

### Why rooms are more reliable than channels

Direct messages have delivery notification feedback in the app to show you if
your message successfully arrived at its destination. Channels only show that
your message was repeated by _someone_. Also, since Meshtastic 2.6, it makes use
of ["next-hop" routing for direct
messages](https://meshtastic.org/blog/meshtastic-2-6-preview/#next-hop-routing-for-dms).
Channels however do not benefit from this improvement.

Sometimes direct messages arrive properly, but the delivery notification doesn't
make it back to you. As additional feedback that you have successfully sent a
message, any messages you send to a room will also be echoed back to you. If
your connection to the bot is poor, this may take a while though, so be patient
before sending again.

Finally, Meshbot will keep trying to send messages to all users in a room
(including the sender) until it receives good delivery notifications. This means
that you may sometimes receive messages multiple times, but it ensures that your
communication is fairly reliable. Even if you move out of range, Meshbot will
remember which messages you missed and as soon as it sees you coming back into
range it will send you the entire history since you left.

## Hosting Meshbot

### Be responsible

There is very little bandwidth available on Meshtastic. If you use this bot, and
especially if you wish to modify it, please make sure it doesn't spam your local
mesh. Make sure it only speaks when spoken to. Et cetera. Be a good neighbour.

### Setup

> **Please note** that this bot has currently **only** been tested on Linux and
> as a Docker image, over TCP. I expect it will probably work over USB and/or on
> MacOS, Windows or Raspberry Pi, but beware there may be dragons 😉 Feel free
> to create an issue if you run into things, but broad support is currently not
> a high priority.

You will need a Meshtastic node and a computer to host the bot. The node and the
computer can either be connected through a USB cable, or [trough your network
over wifi or
ethernet](https://meshtastic.org/docs/configuration/radio/network/).

The former can be super mobile and does not depend on your local network being
up (for example during a power outage). The latter allows you the luxury of
having your node in the best possible spot for reception, while the bot is
running wherever you happen to have compute.

#### Meshtastic node

This software is being developed using a Heltec v3, but any Meshtastic node
should do.

Make sure no other client besides the bot is communicating with the node,
otherwise both clients will be missing messages and things will appear to be
very broken. So disconnect your mobile app and don't make any other connections
to it while the bot is running.

Pro-tips on the Meshtastic side:

- Add a robot emoji (🤖) to your node name to make it clear to other users that
  your node is a bot.
- You can add quick chat messages -- at least in the Android Meshtastic app.
  Adding the commands that the bot accepts (like `/rooms` and `/signal`) as
  quick messages makes them really easily accessible with one click.

#### Computer

Any computer will do as long as it stays on, of course. I run it locally on my
NAS, but even an old Raspberry Pi should work great for the bot. It requires
very few resources. You can run the bot through Docker or directly on the
computer.

Download the appropriate version of the software from the [releases
page](https://github.com/Timendus/meshbot/releases). Edit the `config.json` file
to tell the bot how to connect to your node and how to behave and start the
software. `config.json` should be in the same directory as the software.

For the docker version, mount a directory to `/app/config`. Launch the
container. The first time, if configured correctly, a `config.json` file will be
created in the mounted directory for you to edit. Stop the container, edit the
config file and restart the container.

## Local development

Dependencies:

- Golang
- Git
- make

Then do something like:

```bash
git clone git@github.com:Timendus/meshbot.git
cd meshbot
vi config.json
make
```

And the bot should start.
