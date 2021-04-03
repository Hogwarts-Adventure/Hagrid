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
	GuildID string `json:"guildID"`
	PgLocalhostURL string `json:"pgLocalhostURL"`
	PgDistantURL string `json:"pgDistantURL"`
	IntroReactionID string `json:"introReactionID"`
	IntroReactionRoles []string `json:"introReactionRoles"`
	TicketReactionID string `json:"ticketReactionID"`
	TicketAllowedRoles []string `json:"ticketAllowedRoles"`
	TicketCategoryID string `json:"ticketCategoryID"`
	TicketEmojiID string `json:"ticketEmojiID"`
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
	if g, err := hg.Session.State.Guild(hg.Config.GuildID); err == nil {
		return g // est déjà dans le cache
	}

	g, err := hg.Session.Guild(hg.Config.GuildID) // pas dans le cache => on la fetch
	if err != nil {
		log.Fatal("Le serveur est innaccessible.")
	}
	return g
}

/* Fin Système */

/* Base de données */

type AlluserDbUser struct {
	ID string
	Lang string
}

type Maison struct {
	Points uint16
	DbID int
	Name string
	RoleID string
}

type UsersDbUser struct {
	ID string
	Maison *Maison
}

type AllDbUser struct {
	ID string
	Author *discordgo.Member
	Alluser AlluserDbUser
	Users UsersDbUser
}

/* Fin Base de données */

var (
	MaisonsIdenfiers = map[string]*Maison{
		"GRYFFONDOR": {
			RoleID: "796774549232287754",
			Name: "Gryffondor",
			DbID: 1,
		},
		"POUFSOUFFLE": {
			RoleID: "796775145317859373",
			Name: "Poufsouffle",
			DbID: 3,
		},
		"SERPENTARD": {
			RoleID: "796774926383972383",
			Name: "Serpentard",
			DbID: 2,
		},
		"SERDAIGLE": {
			RoleID: "796775403707826227",
			Name: "Serdaigle",
			DbID: 4,
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
			if MaisonsIdenfiers[m].DbID == val.(int) {
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

func (hg *Hagrid) GetUserDb(userID string) *AllDbUser {
	author, _ := hg.GetMember(userID)
	userDb := AllDbUser{
		ID: userID,
		Author: author,
		Users: UsersDbUser{
			ID: userID,
			Maison: &Maison{},
		},
		Alluser: AlluserDbUser{
			ID: userID,
		},
	}
	_ = Hg.DB.QueryRow(context.Background(), "SELECT users.maison, alluser.lang FROM users INNER JOIN alluser ON users.id = alluser.id WHERE users.id = $1", userID).Scan(&userDb.Users.Maison.Name, &userDb.Alluser.Lang)
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

func (hg *Hagrid) GetMember(memberID string) (*discordgo.Member, error) {
	m, err := hg.Session.State.Member(hg.Config.GuildID, memberID)
	if err != nil {
		var nErr error
		if m, nErr = hg.Session.GuildMember(hg.Config.GuildID, memberID); nErr != nil {
			return nil, errors.New("no member")
		}
	}
	return m, nil
}

func (hg *Hagrid) GetChannel(channelID string) (*discordgo.Channel, error) {
	c, err := hg.Session.State.Channel(channelID)
	if err != nil {
		var nErr error
		if c, nErr = hg.Session.Channel(channelID); nErr != nil {
			return nil, errors.New("no channel")
		}
	}
	return c, nil
}

/* Fin Base de données */