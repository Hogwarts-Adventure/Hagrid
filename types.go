package main

import (
	"context"
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"
)

/* Système */

type Config struct {
	Token string `json:"token"`
	Prefix string `json:"prefix"`
	DevPrefix string `json:"devPrefix"`
	GuildId string `json:"guildId"`
	PgLocalhostURL string `json:"pgLocalhostURL"`
	PgDistantURL string `json:"pgDistantURL"`
}

type Hagrid struct {
	Session *discordgo.Session
	Config *Config
	DB *pgx.Conn
}

func NewHagrid() Hagrid {
	hg := Hagrid{}
	hg.readConfig()
	rand.Seed(time.Now().UnixNano()) // initialiser rand
	return hg
}

func (hg *Hagrid) readConfig() {
	jsonF, err := os.Open("res/config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer jsonF.Close()

	bVal, _ := ioutil.ReadAll(jsonF)
	_ = json.Unmarshal(bVal, &hg.Config)

	if runtime.GOOS == "windows" {
		hg.Config.Prefix = hg.Config.DevPrefix
	}
}

func (hg *Hagrid) ConnectDb() {
	pgUrl := hg.Config.PgLocalhostURL

	if runtime.GOOS == "windows" {
		pgUrl = hg.Config.PgDistantURL
	}

	conn, err := pgx.Connect(context.Background(), pgUrl)
	if err != nil {
		log.Fatal(err)
	}

	hg.DB = conn
}

func (hg *Hagrid) GetGuild() *discordgo.Guild {
	g, err := hg.Session.Guild(hg.Config.GuildId)
	if err != nil {
		log.Fatal("Le serveur est innaccessible.")
	}
	return g
}

/* Fin Système */

/* Base de données */

type AlluserDbUser struct {
	Id string
	Lang string
}

type Maison struct {
	Id uint8
	Points uint16
	Nom string
	Identifiers MaisonIdentifier
}

type UsersDbUser struct {
	Id string
	Maison string
}

type AllDbUser struct {
	Id string
	Author *discordgo.User
	Alluser AlluserDbUser
	Users UsersDbUser
}

/* Fin Base de données */

type MaisonIdentifier struct {
	DbId uint8
	Name string
	RoleId string
}

var (
	Maisons = map[string]MaisonIdentifier{
		"GRYFFONDOR": {
			RoleId: "796774549232287754",
		},
		"POUFSOUFFLE": {
			RoleId: "796775145317859373",
		},
		"SERPENTARD": {
			RoleId: "796774926383972383",
		},
		"SERDAIGLE": {
			RoleId: "796775403707826227",
		},
	}
	MaisonsNames = func() []string {
		keys := make([]string, 0, 4)
		for k := range Maisons {
			keys = append(keys, k)
		}
		return keys
	}()
)

func GetMaison(val interface{}) *Maison {
	toRet := &Maison{}
	switch val.(type) {
	case string:
		name := strings.ToUpper(val.(string))
		if pos := StringSliceFind(MaisonsNames, name); pos != -1 {
			toRet.Identifiers = Maisons[name]
		}
	}
	return toRet
}