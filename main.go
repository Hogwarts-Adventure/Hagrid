package main

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	CheckHouseCooldown       = time.Second * 30
	FirewallCooldown         = time.Second * 20
	TicketChannelPermissions = int64(discordgo.PermissionViewChannel + discordgo.PermissionSendMessages + discordgo.PermissionAttachFiles + discordgo.PermissionReadMessageHistory + discordgo.PermissionUseExternalEmojis + discordgo.PermissionAddReactions)
)

var Hg = NewHagrid()

func main() {
	dg, err := discordgo.New("Bot " + Hg.Config.Token)
	if err != nil {
		log.Fatal("Erreur création du client")
	}

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAll)

	Hg.Session = dg

	Hg.ConnectDb()
	defer Hg.DB.Close(context.Background())

	dg.AddHandler(ready)
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)
	dg.AddHandler(messageReactionRemove)
	dg.AddHandler(guildMemberAdd)
	dg.AddHandler(guildMemberRemove)

	err = dg.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer dg.Close()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func messageCreate(_ *discordgo.Session, m *discordgo.MessageCreate) {
	// vérifications de base
	if m.GuildID != Hg.Config.GuildID || m.Author.Bot {
		return
	}

	// vérifier rôle maison
	if StringSliceContains(Hg.CheckHouseCooldowns, m.Author.ID) {
		_ = Hg.CheckUserHouseRole(m.Author.ID, m.Member.Roles)

		Hg.CheckHouseCooldowns = append(Hg.CheckHouseCooldowns, m.Author.ID)
		time.AfterFunc(CheckHouseCooldown, func() {
			Hg.CheckHouseCooldowns = StringSliceRemoveTarget(Hg.CheckHouseCooldowns, m.Author.ID)
		})
	}

	channel, err := Hg.GetChannel(m.ChannelID)
	if err != nil || channel.Type == discordgo.ChannelTypeDM || channel.Type == discordgo.ChannelTypeGroupDM {
		return
	}
}

func messageReactionAdd(session *discordgo.Session, reaction *discordgo.MessageReactionAdd) {
	if reaction.GuildID != Hg.Config.GuildID {
		return
	}

	player := Hg.GetPlayer(reaction.UserID)
	if player.Member.User.Bot {
		return
	}

	// firewall
	if reaction.MessageID == Hg.Config.IntroReactionID {
		HandleFirewall(session, reaction, player)
	} else if reaction.MessageID == Hg.Config.TicketReactionID && reaction.Emoji.ID == Hg.Config.TicketEmojiID { // ticket support
		HandleTicketCreation(session, reaction, player)
	} else if reaction.ChannelID == Hg.Config.AssignableRolesChannelID {
		HandleAssignableRolesReactionAdd(session, reaction, player)
	}
}

func messageReactionRemove(session *discordgo.Session, reaction *discordgo.MessageReactionRemove) {
	if reaction.GuildID != Hg.Config.GuildID {
		return
	}

	player := Hg.GetPlayer(reaction.UserID)

	if reaction.ChannelID == Hg.Config.AssignableRolesChannelID {
		HandleAssignableRolesReactionRemove(session, reaction, player)
	}
}

func guildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.GuildID != Hg.Config.GuildID {
		return
	}
	user := Hg.GetPlayer(m.User.ID)
	_, _ = s.ChannelMessageSend(Hg.Config.TrafficChannelID,
		strings.ReplaceAll(
			strings.ReplaceAll(
				Hg.GetLang("welcomeMessage", user.Lang), "{{mention}}", m.Mention()),
			"{{count}}", strconv.Itoa(Hg.GetGuild().MemberCount)))
}

func guildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.GuildID != Hg.Config.GuildID {
		return
	}
	_, _ = s.ChannelMessageSend(Hg.Config.TrafficChannelID,
		strings.ReplaceAll(Hg.GetLang("byeMessage", "fr"), "{{username}}", m.Mention()+"(`"+m.User.String()+"`)"))
}

func ready(s *discordgo.Session, _ *discordgo.Ready) {
	_ = s.UpdateGameStatus(0, "vous surveiller bande d'ingrats -_-")
	fmt.Println(time.Now().Format("02-Jan-2006: 15h04m05s"), "Bot connecté !")
}
