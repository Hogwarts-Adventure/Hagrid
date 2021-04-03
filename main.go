package main

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
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

	Hg.Session = dg

	Hg.ConnectDb()
	defer Hg.DB.Close(context.Background())

	dg.AddHandler(ready)
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)

	err = dg.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer dg.Close()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	channel, err := s.State.Channel(m.ChannelID) // prend dans le cache
	if err != nil { // si pas dans le cache
		var nErr error
		if channel, nErr = s.Channel(m.ChannelID); nErr != nil {
			return
		}
	}

	if m.Author.Bot || channel.Type == discordgo.ChannelTypeDM || channel.Type == discordgo.ChannelTypeGroupDM {
		return
	}

	userDb := Hg.GetUserDb(m.Author.ID)
	
	if userDb.Users.Maison.Name != "" { // si il a une maison
		userDb.Users.Maison = Hg.GetMaison(userDb.Users.Maison.Name, false)
		house := userDb.Users.Maison
		for _, h := range MaisonsIdenfiers {
			if h.RoleId == house.RoleId && StringSliceFind(m.Member.Roles, house.RoleId) == -1 { // si c'est sa maison et qu'il n'a pas le rôle
				_ = s.GuildMemberRoleAdd(m.GuildID, m.Author.ID, h.RoleId)
			} else if h.RoleId != house.RoleId && StringSliceFind(m.Member.Roles, house.RoleId) != -1 { // si ce n'est pas sa maison mais qu'il a le rôle
				_ = s.GuildMemberRoleRemove(m.GuildID, m.Author.ID, h.RoleId)
			}
		}
	}
}

func messageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	member, err := Hg.GetMember(r.UserID)
	if err != nil || member.User.Bot {
		return
	}

	if r.MessageID == Hg.Config.IntroReactionId { // reaction rôle
		for _, id := range Hg.Config.IntroReactionRoles {
			if StringSliceFind(member.Roles, id) == -1 { // si il ne l'a pas
				_ = s.GuildMemberRoleAdd(r.GuildID, r.UserID, id)
			}
		}
	} else if r.MessageID == Hg.Config.TicketReactionId { // ticket support
		// vérifie si salon n'existe pas déjà
		channels, _ := s.GuildChannels(r.GuildID)
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
			ParentID: Hg.Config.TicketCategoryId,
			PermissionOverwrites: []*discordgo.PermissionOverwrite{
				{
					ID: r.GuildID,
					Deny: discordgo.PermissionViewChannel,
				},
				{
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
		_, _ = s.ChannelMessageSend(channel.ID, strings.Replace(
			Hg.GetLang("afterTicketMention", "fr"),
			"(uid)",
			user.Id,
			-1),
		)
		_ = s.MessageReactionsRemoveAll(r.ChannelID, r.MessageID)
		_ = s.MessageReactionAdd(r.ChannelID, r.MessageID, r.Emoji.ID)
	}
}

func ready(s *discordgo.Session, _ *discordgo.Ready) {
	_ = s.UpdateGameStatus(0, "vous surveiller bande d'ingrats -_-")
	fmt.Println("Bot connecté !")
}