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
		},
		Alluser: AlluserDbUser{
			Id: m.Author.ID,
		},
	}
	err = hg.DB.QueryRow(context.Background(), "SELECT users.maison, alluser.lang FROM users INNER JOIN alluser ON users.id = alluser.id WHERE id = $1", m.Author.ID).Scan(&userDb.Users.Maison, &userDb.Alluser.Lang)

	if userDb.Users.Maison
}

func ready(s *discordgo.Session, _ *discordgo.Ready) {
	_ = s.UpdateGameStatus(0, "vous surveiller bande d'ingrats -_-")
	fmt.Println("Bot connecté !")
}