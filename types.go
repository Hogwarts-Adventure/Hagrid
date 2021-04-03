package main

import (
	"context"
	"encoding/json"
	"errors"
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
	TicketReactionId string `json:"ticketReactionId"`
	TicketAllowedRoles []string `json:"ticketAllowedRoles"`
	TicketCategoryId string `json:"ticketCategoryId"`
}

type Hagrid struct {
	Session *discordgo.Session
	Config *Config
	DB *pgx.Conn
	Lang map[string]interface{}
}

func NewHagrid() Hagrid {
	hg := Hagrid{}
	hg.readConfig()
	hg.readLanguages()
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

func (hg *Hagrid) readLanguages() {
	d, err := os.Open("res/lang.json")
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	data, _ := ioutil.ReadAll(d)
	_ = json.Unmarshal(data, &hg.Lang)
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
	if g, err := hg.Session.State.Guild(hg.Config.GuildId); err == nil {
		return g // est déjà dans le cache
	}

	g, err := hg.Session.Guild(hg.Config.GuildId) // pas dans le cache => on la fetch
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
	Author *discordgo.Member
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

func (hg *Hagrid) GetUserDb(userId string) *AllDbUser {
	author, _ := hg.GetMember(userId)
	userDb := AllDbUser{
		Id: userId,
		Author: author,
		Users: UsersDbUser{
			Id: userId,
			Maison: &Maison{},
		},
		Alluser: AlluserDbUser{
			Id: userId,
		},
	}
	_ = Hg.DB.QueryRow(context.Background(), "SELECT users.maison, alluser.lang FROM users INNER JOIN alluser ON users.id = alluser.id WHERE users.id = $1", userId).Scan(&userDb.Users.Maison.Name, &userDb.Alluser.Lang)
	if userDb.Alluser.Lang == "" {
		userDb.Alluser.Lang = "fr"
	}
	return &userDb
}

// len(lang) != 2 => fetch utilisateur
func (hg *Hagrid) GetLang(search string, lang string) string {
	if len(lang) != 2 {
		lang = hg.GetUserDb(lang).Alluser.Lang
	}
	s := hg.Lang[search]
	if s == nil {
		return "error lang"
	}
	return s.(map[string]interface{})[lang].(string)
}

func (hg *Hagrid) GetMember(memberId string) (*discordgo.Member, error) {
	m, err := hg.Session.State.Member(hg.Config.GuildId, memberId)
	if err != nil {
		var nErr error
		if m, nErr = hg.Session.GuildMember(hg.Config.GuildId, memberId); nErr != nil {
			return nil, errors.New("no member")
		}
	}
	return m, nil
}

func (hg *Hagrid) GetChannel(channelId string) (*discordgo.Channel, error) {
	c, err := hg.Session.State.Channel(channelId)
	if err != nil {
		var nErr error
		if c, nErr = hg.Session.Channel(channelId); nErr != nil {
			return nil, errors.New("no channel")
		}
	}
	return c, nil
}

/* Fin Base de données */