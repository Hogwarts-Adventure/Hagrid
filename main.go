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
	if m.GuildID != Hg.Config.GuildID || StringSliceFind(Hg.CheckCooldowns, m.Author.ID) != -1 {
		return
	}

	channel, err := Hg.GetChannel(m.ChannelID)
	if err != nil || m.Author.Bot || channel.Type == discordgo.ChannelTypeDM || channel.Type == discordgo.ChannelTypeGroupDM {
		return
	}

	_ = Hg.CheckUserHouseRole(m.Author.ID, m.Member.Roles)

	Hg.CheckCooldowns = append(Hg.CheckCooldowns, m.Author.ID)
	time.AfterFunc(time.Second * 20, func() {
		Hg.CheckCooldowns = StringSliceRemove(Hg.CheckCooldowns, StringSliceFind(Hg.CheckCooldowns, m.Author.ID))
	})
}

func messageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.GuildID != Hg.Config.GuildID {
		return
	}

	member, err := Hg.GetMember(r.UserID)
	if err != nil || member.User.Bot {
		return
	}

	if r.MessageID == Hg.Config.IntroReactionID { // reaction rôle
		for _, id := range Hg.Config.IntroReactionRoles {
			if StringSliceFind(member.Roles, id) == -1 { // si il ne l'a pas
				_ = s.GuildMemberRoleAdd(r.GuildID, r.UserID, id)
			}
		}
	} else if r.MessageID == Hg.Config.TicketReactionID && r.Emoji.ID == Hg.Config.TicketEmojiID { // ticket support
		// vérifie si salon n'existe pas déjà
		channels, _ := s.GuildChannels(r.GuildID)

		defer s.MessageReactionAdd(r.ChannelID, r.MessageID, r.Emoji.APIName())
		defer s.MessageReactionsRemoveAll(r.ChannelID, r.MessageID)

		for _, channel := range channels {
			if strings.HasPrefix(channel.Topic, r.UserID) { // salon support existe déjà
				_, _ = s.ChannelMessageSend(channel.ID, "<@" + r.UserID + "> " + Hg.GetLang("ticketChannelAlreadyExists", r.UserID))
				return
			}
		}

		user := Hg.GetUserDb(r.UserID)
		perms := int64(discordgo.PermissionViewChannel + discordgo.PermissionSendMessages + discordgo.PermissionAttachFiles + discordgo.PermissionReadMessageHistory + discordgo.PermissionUseExternalEmojis + discordgo.PermissionAddReactions)
		createData := discordgo.GuildChannelCreateData{
			Name: member.User.Username,
			Type: discordgo.ChannelTypeGuildText,
			Topic: r.UserID,
			ParentID: Hg.Config.TicketCategoryID,
			PermissionOverwrites: []*discordgo.PermissionOverwrite{
				{
					ID: r.GuildID,
					Deny: discordgo.PermissionViewChannel,
				},
				{
					Type: discordgo.PermissionOverwriteTypeMember,
					ID: r.UserID,
					Allow: perms,
				},
			},
		}

		// ajoute pour les rôles autorisés
		for _, role := range Hg.Config.TicketAllowedRoles {
			createData.PermissionOverwrites = append(createData.PermissionOverwrites, &discordgo.PermissionOverwrite{
				ID: role,
				Allow: perms,
			})
		}
		// crée le salon
		channel, e := s.GuildChannelCreateComplex(r.GuildID, createData)
		if e != nil {
			fmt.Println(e)
			// si erreur, message => supprimé 10s après envoie
			m, _ := s.ChannelMessageSend(r.ChannelID, Hg.GetLang("ticketError", user.Alluser.Lang))
			time.AfterFunc(time.Second * 10, func(){
				_ = s.ChannelMessageDelete(r.ChannelID, m.ID)
			})
			return
		}
		// sinon envoie embed
		_, _ = s.ChannelMessageSendEmbed(channel.ID, &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				Name: member.User.Username,
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: s.State.User.Username,
			},
			Description: Hg.GetLang("ticketMessage", user.Alluser.Lang),
		})
		_, _ = s.ChannelMessageSend(channel.ID, strings.ReplaceAll(
			Hg.GetLang("afterTicketMention", "fr"),
			"(uid)",
			user.ID),
		)
	}/* else if r.MessageID ==  { // rôle "en service"

	}*/
}

func guildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.GuildID != Hg.Config.GuildID {
		return
	}
	user := Hg.GetUserDb(m.User.ID)
	_, _ = s.ChannelMessageSend(Hg.Config.TrafficChannelID,
		strings.ReplaceAll(
			strings.ReplaceAll(
				Hg.GetLang("welcomeMessage", user.Alluser.Lang), "{{mention}}", m.Mention()),
				"{{count}}", strconv.Itoa(Hg.GetGuild().MemberCount)))
}

func guildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.GuildID != Hg.Config.GuildID {
		return
	}
	_,_ = s.ChannelMessageSend(Hg.Config.TrafficChannelID,
		strings.ReplaceAll(Hg.GetLang("byeMessage", "fr"), "{{username}}", m.Mention() + "(`" + m.User.String() + "`)"))
}

func ready(s *discordgo.Session, _ *discordgo.Ready) {
	_ = s.UpdateGameStatus(0, "vous surveiller bande d'ingrats -_-")
	fmt.Println(time.Now().Format("02-Jan-2006: 15h04m05s"), "Bot connecté !")
}