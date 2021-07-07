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
	"strconv"
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

	PremiumRoleID string `json:"premiumRoleID"`

	TrafficChannelID string `json:"trafficChannelID"`

	EnServiceMessageID string `json:"enServiceMessageID"`
	EnServiceEmojiID  string `json:"enServiceEmojiID"`
	EnServiceRoleID string `json:"enServiceRoleID"`
}

type Hagrid struct {
	Session *discordgo.Session
	Config *Config
	DB *pgx.Conn
	Lang map[string]interface{}
	CheckCooldowns []string
}

func NewHagrid() Hagrid {
	hg := Hagrid{}
	hg.readConfig()
	hg.readLanguages()
	rand.Seed(time.Now().UnixNano()) // initialiser rand
	hg.CheckCooldowns = []string{}
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
	DatePremium time.Time
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

	var prem string
	_ = Hg.DB.QueryRow(context.Background(), `SELECT users.maison, users."datePremium", alluser.lang FROM users INNER JOIN alluser ON users.id = alluser.id WHERE users.id = $1`, userID).Scan(&userDb.Users.Maison.Name, &prem, &userDb.Alluser.Lang)
	if userDb.Alluser.Lang == "" {
		userDb.Alluser.Lang = "fr"
	}
	premInt, err := strconv.ParseInt(prem, 10, 64)
	if err != nil || premInt == 0 {
		userDb.Users.DatePremium = time.Unix(0, 0)
	} else {
		userDb.Users.DatePremium = time.Unix(premInt/1000, 0)
	}
	return &userDb
}

// GetLang : len(lang) != 2 => fetch utilisateur
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

func (hg *Hagrid) CheckUserHouseRole(userID string, memberRoles []string) error {
	userDb := Hg.GetUserDb(userID)
	if userDb.Users.Maison.Name != "" { // si il a une maison
		userDb.Users.Maison = Hg.GetMaison(userDb.Users.Maison.Name, false)
		house := userDb.Users.Maison
		for _, h := range MaisonsIdenfiers {
			if h.RoleID == house.RoleID && StringSliceFind(memberRoles, house.RoleID) == -1 { // si c'est sa maison et qu'il n'a pas le rôle
				_ = hg.Session.GuildMemberRoleAdd(hg.Config.GuildID, userID, h.RoleID)
			} else if h.RoleID != house.RoleID && StringSliceFind(memberRoles, house.RoleID) != -1 { // si ce n'est pas sa maison mais qu'il a le rôle
				_ = hg.Session.GuildMemberRoleRemove(hg.Config.GuildID, userID, h.RoleID)
			}
		}
		if userDb.Users.DatePremium != time.Unix(0, 0) {
			if userDb.Users.DatePremium.Before(time.Now()) {
				_, _ = Hg.DB.Exec(context.Background(), `UPDATE users SET "datePremium" = '' WHERE id = $1`, userDb.ID)
				if pos := StringSliceFind(userDb.Author.Roles, Hg.Config.PremiumRoleID); pos != -1 {
					_ = hg.Session.GuildMemberRoleRemove(hg.Config.GuildID, userDb.ID, Hg.Config.PremiumRoleID)
				}
			} else {
				if pos := StringSliceFind(userDb.Author.Roles, Hg.Config.PremiumRoleID); pos == -1 {
					_ = hg.Session.GuildMemberRoleAdd(hg.Config.GuildID, userDb.ID, Hg.Config.PremiumRoleID)
				}
			}
		}
	}

	return nil
}