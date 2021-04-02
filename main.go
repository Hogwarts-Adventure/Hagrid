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

var hg = NewHagrid()

func main() {
	dg, err := discordgo.New("Bot " + hg.Config.Token)
	if err != nil {
		log.Fatal("Erreur création du client")
	}

	hg.Session = dg

	hg.ConnectDb()
	defer hg.DB.Close(context.Background())

	dg.AddHandler(ready)
	dg.AddHandler(messageCreate)

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
	_ = hg.DB.QueryRow(context.Background(), "SELECT users.maison, alluser.lang FROM users INNER JOIN alluser ON users.id = alluser.id WHERE users.id = $1", m.Author.ID).Scan(&userDb.Users.Maison.Name, &userDb.Alluser.Lang)

	
	if userDb.Users.Maison.Name != "" { // si il a une maison
		userDb.Users.Maison = hg.GetMaison(userDb.Users.Maison.Name, false)
		house := userDb.Users.Maison
		for r := range MaisonsIdenfiers {
			rid := MaisonsIdenfiers[r].RoleId
			if rid == house.RoleId && StringSliceFind(m.Member.Roles, house.RoleId) == -1 { // si c'est sa maison et qu'il n'a pas le rôle
				_ = s.GuildMemberRoleAdd(m.GuildID, m.Author.ID, rid)
			} else if rid != house.RoleId && StringSliceFind(m.Member.Roles, house.RoleId) != -1 { // si ce n'est pas sa maison mais qu'il a le rôle
				_ = s.GuildMemberRoleRemove(m.GuildID, m.Author.ID, rid)
			}
		}
	}
}

func ready(s *discordgo.Session, _ *discordgo.Ready) {
	_ = s.UpdateGameStatus(0, "vous surveiller bande d'ingrats -_-")
	fmt.Println("Bot connecté !")
}