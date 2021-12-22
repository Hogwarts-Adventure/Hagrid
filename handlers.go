package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"strings"
	"time"
)

func HandleFirewall(session *discordgo.Session, reaction *discordgo.MessageReactionAdd, player *Player) {
	introCooldown := Hg.OtherCooldowns["intro"]

	if !StringSliceContains(introCooldown, reaction.UserID) { // pas dans le cooldown
		Hg.OtherCooldowns["intro"] = append(Hg.OtherCooldowns["intro"], reaction.UserID)
		time.AfterFunc(FirewallCooldown, func() {
			Hg.CheckHouseCooldowns = StringSliceRemoveTarget(Hg.CheckHouseCooldowns, reaction.UserID)
		})

		fmt.Printf("Reaction firewall de: %s\n", player.Member.User.String())
		for _, id := range Hg.Config.IntroReactionRoles {
			if !StringSliceContains(player.Member.Roles, id) { // si il ne l'a pas
				_ = session.GuildMemberRoleAdd(reaction.GuildID, reaction.UserID, id)
				time.Sleep(time.Second) // pause 1 seconde entre chaque attribution de rôle
			}
		}
	}
}

func HandleTicketCreation(session *discordgo.Session, reaction *discordgo.MessageReactionAdd, player *Player) {
	channels, _ := session.GuildChannels(reaction.GuildID)

	defer session.MessageReactionAdd(reaction.ChannelID, reaction.MessageID, reaction.Emoji.APIName())
	defer session.MessageReactionsRemoveAll(reaction.ChannelID, reaction.MessageID)

	{ // vérifie que le salon n'existe pas déjà
		found := false
		for i := 0; i < len(channels) && !found; i++ {
			channel := channels[i]
			if strings.HasPrefix(channel.Topic, reaction.UserID) { // salon support existe déjà
				_, _ = session.ChannelMessageSend(channel.ID, "<@"+reaction.UserID+"> "+Hg.GetLang("ticketChannelAlreadyExists", reaction.UserID))
				found = true
			}
		}
		if found {
			return
		}
	}

	createData := discordgo.GuildChannelCreateData{
		Name:     player.Member.User.Username,
		Type:     discordgo.ChannelTypeGuildText,
		Topic:    reaction.UserID,
		ParentID: Hg.Config.TicketCategoryID,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{
				Type: discordgo.PermissionOverwriteTypeRole,
				ID:   reaction.GuildID,
				Deny: discordgo.PermissionViewChannel,
			},
			{
				Type:  discordgo.PermissionOverwriteTypeMember,
				ID:    reaction.UserID,
				Allow: TicketChannelPermissions,
			},
		},
	}

	// ajoute pour les rôles autorisés
	for _, role := range Hg.Config.TicketAllowedRoles {
		createData.PermissionOverwrites = append(createData.PermissionOverwrites, &discordgo.PermissionOverwrite{
			Type:  discordgo.PermissionOverwriteTypeRole,
			ID:    role,
			Allow: TicketChannelPermissions,
		})
	}

	// crée le salon
	channel, e := session.GuildChannelCreateComplex(reaction.GuildID, createData)

	time.Sleep(time.Second * 3)

	if e != nil {
		fmt.Println(e)
		// si erreur, message => supprimé 10s après envoie
		m, _ := session.ChannelMessageSend(reaction.ChannelID, Hg.GetLang("ticketError", player.Lang))
		time.AfterFunc(time.Second*10, func() {
			_ = session.ChannelMessageDelete(reaction.ChannelID, m.ID)
		})
		return
	} else {
		// sinon envoie embed
		_, _ = session.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
			Embed: &discordgo.MessageEmbed{
				Author: &discordgo.MessageEmbedAuthor{
					Name:    player.Member.User.Username,
					IconURL: player.Member.User.AvatarURL(""),
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text: session.State.User.Username,
				},
				Description: Hg.GetLang("ticketMessage", player.Lang),
			},
			Content: strings.ReplaceAll(Hg.GetLang("afterTicketMention", "fr"), "{{uid}}", player.ID),
		})

		fmt.Printf("Creation de ticket pour %s\n", player.Member.User.String())
	}
}

func HandleAssignableRolesReactionAdd(session *discordgo.Session, reaction *discordgo.MessageReactionAdd, player *Player) {
	roleID, notDefined := MapGetKeyByValue(&Hg.AssignableRoles, reaction.Emoji.ID)

	if notDefined != nil {
		fmt.Println("Mauvais emoji dans le salon des roles assignables:")
		fmt.Printf("\t%s a reagi avec: \"%s\"", player.Member.User.String(), reaction.Emoji.ID)
	} else {
		if !StringSliceContains(player.Member.Roles, roleID) { // si n'a pas déjà le rôle
			guildRoles := Hg.GetGuild().Roles
			found := false
			for i := 0; i < len(guildRoles) && !found; i++ {
				found = roleID == guildRoles[i].ID
			}
			if !found { // rôle introuvable
				fmt.Printf("Le role avec l'ID %s est introuvable\n", roleID)
				if dm, err := session.UserChannelCreate(reaction.UserID); err == nil {
					_, _ = session.ChannelMessageSend(dm.ID, strings.ReplaceAll(player.GetLang("roleError"), "{{id}}", roleID))
				}
			} else {
				_ = session.GuildMemberRoleAdd(reaction.GuildID, reaction.UserID, roleID)
			}
		}
	}
}

func HandleAssignableRolesReactionRemove(session *discordgo.Session, reaction *discordgo.MessageReactionRemove, player *Player) {
	if roleID, notFound := MapGetKeyByValue(&Hg.AssignableRoles, reaction.Emoji.ID); notFound == nil { // rôle existe
		if StringSliceContains(player.Member.Roles, roleID) { // possède le rôle
			_ = session.GuildMemberRoleRemove(reaction.GuildID, reaction.UserID, roleID)
		}
	}
}
