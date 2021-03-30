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

	err = dg.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer dg.Close()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func ready(s *discordgo.Session, _ *discordgo.Ready) {
	_ = s.UpdateGameStatus(0, "vous surveiller bande d'ingrats -_-")
	fmt.Println("Bot connecté !")
}