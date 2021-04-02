package main

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"syscall"
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
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		return
	}

	if m.Author.Bot || channel.Type == discordgo.ChannelTypeDM || channel.Type == discordgo.ChannelTypeGroupDM {
		return
	}

	userDb := AllDbUser{
		Id: m.Author.ID,
		Author: m.Author,
		Users: UsersDbUser{
			Id: m.Author.ID,
			Maison: &Maison{},
		},
		Alluser: AlluserDbUser{
			Id: m.Author.ID,
		},
	}
	_ = Hg.DB.QueryRow(context.Background(), "SELECT users.maison, alluser.lang FROM users INNER JOIN alluser ON users.id = alluser.id WHERE users.id = $1", m.Author.ID).Scan(&userDb.Users.Maison.Name, &userDb.Alluser.Lang)

	
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
	if r.MessageID == Hg.Config.IntroReactionId {
		m, _ := s.GuildMember(r.GuildID, r.UserID)
		for _, id := range Hg.Config.IntroReactionRoles {
			if StringSliceFind(m.Roles, id) == -1 { // si il ne l'a pas
				_ = s.GuildMemberRoleAdd(r.GuildID, r.UserID, id)
			}
		}
	}
}

func ready(s *discordgo.Session, _ *discordgo.Ready) {
	_ = s.UpdateGameStatus(0, "vous surveiller bande d'ingrats -_-")
	fmt.Println("Bot connecté !")
}