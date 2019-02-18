package main

import (
	"log"

	"github.com/rivo/tview"
	"github.com/rumblefrog/discordgo"
)

func messageAck(s *discordgo.Session, a *discordgo.MessageAck) {
	// Sets ReadState to the message you read
	for _, c := range d.State.ReadState {
		if c.ID == a.ChannelID {
			c.LastMessageID = a.MessageID
		}
	}

	// update
	checkReadState()
}

func checkReadState() {
	var guildSettings *discordgo.UserGuildSettings

	if d.State == nil {
		return
	}

	if d.State.Settings == nil {
		return
	}

	if guildView == nil {
		return
	}

	changed := false

	root := guildView.GetRoot()
	if root == nil {
		return
	}

	root.Walk(func(node, parent *tview.TreeNode) bool {
		if parent == nil {
			return true
		}

		reference := node.GetReference()
		if reference == nil {
			return true
		}

		id, ok := reference.(int64)
		if !ok {
			return true
		}

		c, err := d.State.Channel(id)
		if err != nil {
			return true
		}

		if guildSettings == nil || guildSettings.GuildID != c.GuildID {
			guildSettings = getGuildFromSettings(c.GuildID)
		}

		var (
			chSettings = getChannelFromGuildSettings(c.ID, guildSettings)
			name       = "[::d]" + c.Name + "[::-]"

			chMuted = settingChannelIsMuted(chSettings)
			guMuted = settingGuildIsMuted(guildSettings)
		)

		if isUnread(c) && !chMuted && !guMuted {
			changed = true

			name = "[::b]" + c.Name + "[::-]"

			g, ok := parent.GetReference().(string)
			if ok {
				parent.SetText("[::b]" + g + "[::-]")
			}
		}

		node.SetText(name)

		return true
	})

	if changed == true {
		app.Draw()
	}
}

// true if channelID has unread msgs
func isUnread(ch *discordgo.Channel) bool {
	for _, c := range d.State.ReadState {
		if c.ID == ch.ID && c.LastMessageID != ch.LastMessageID {
			return true
		}
	}

	return false
}

var lastAck string

func ackMe(c *discordgo.Channel, m *discordgo.Message) {
	// triggers messageAck
	ack, err := d.ChannelMessageAck(c.ID, m.ID, lastAck)

	if err != nil {
		log.Println(err)
		return
	}

	lastAck = ack.Token
}
