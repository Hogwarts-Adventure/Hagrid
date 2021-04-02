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
	IntroReactionId string `json:"introReactionId"`
	IntroReactionRoles []string `json:"introReactionRoles"`
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

func (hg *Hagrid) IsDevVersion() bool {
	return runtime.GOOS == "windows"
}

func (hg *Hagrid) readConfig() {
	jsonF, err := os.Open("res/config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer jsonF.Close()

	bVal, _ := ioutil.ReadAll(jsonF)
	_ = json.Unmarshal(bVal, &hg.Config)

	if hg.IsDevVersion() {
		hg.Config.Prefix = hg.Config.DevPrefix
	}
}

func (hg *Hagrid) ConnectDb() {
	pgUrl := hg.Config.PgLocalhostURL

	if hg.IsDevVersion() {
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
	Points uint16
	DbId int
	Name string
	RoleId string
}

type UsersDbUser struct {
	Id string
	Maison *Maison
}

type AllDbUser struct {
	Id string
	Author *discordgo.User
	Alluser AlluserDbUser
	Users UsersDbUser
}

/* Fin Base de données */

var (
	MaisonsIdenfiers = map[string]*Maison{
		"GRYFFONDOR": {
			RoleId: "796774549232287754",
			Name: "Gryffondor",
			DbId: 1,
		},
		"POUFSOUFFLE": {
			RoleId: "796775145317859373",
			Name: "Poufsouffle",
			DbId: 3,
		},
		"SERPENTARD": {
			RoleId: "796774926383972383",
			Name: "Serpentard",
			DbId: 2,
		},
		"SERDAIGLE": {
			RoleId: "796775403707826227",
			Name: "Serdaigle",
			DbId: 4,
		},
	}
	MaisonsNames = func() []string {
		keys := make([]string, 0, 4)
		for k := range MaisonsIdenfiers {
			keys = append(keys, k)
		}
		return keys
	}()
)

func (hg *Hagrid) GetMaison(val interface{}, queryDb bool) *Maison {
	name := ""
	switch val.(type) {
	case string:
		n := strings.ToUpper(val.(string))
		if pos := StringSliceFind(MaisonsNames, n); pos != -1 {
			name = n
		}
		break
	case int:
		for m := range MaisonsIdenfiers {
			if MaisonsIdenfiers[m].DbId == val.(int) {
				name = m
			}
		}
		break
	}
	if name == "" {
		return nil
	}

	toRet := &Maison{}
	toRet = MaisonsIdenfiers[name]
	if queryDb {
		_ = hg.DB.QueryRow(context.Background(), "SELECT points FROM maisons WHERE nom = $1", name).Scan(&toRet.Points)
	}
	return toRet
}