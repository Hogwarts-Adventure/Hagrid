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
	"time"
)

/* Système */

type Config struct {
	Token                    string   `json:"token"`
	Prefix                   string   `json:"prefix"`
	DevPrefix                string   `json:"devPrefix"`
	GuildID                  string   `json:"guildId"`
	PgLocalhostURL           string   `json:"pgLocalhostURL"`
	PgDistantURL             string   `json:"pgDistantURL"`
	IntroReactionID          string   `json:"introReactionId"`
	IntroReactionRoles       []string `json:"introReactionRoles"`
	TicketReactionID         string   `json:"ticketReactionId"`
	TicketAllowedRoles       []string `json:"ticketAllowedRoles"`
	TicketCategoryID         string   `json:"ticketCategoryId"`
	TicketEmojiID            string   `json:"ticketEmojiId"`
	PremiumRoleID            string   `json:"premiumRoleId"`
	TrafficChannelID         string   `json:"trafficChannelId"`
	AssignableRolesChannelID string   `json:"assignableRolesChannelId"`
}

type Hagrid struct {
	Session             *discordgo.Session
	Config              *Config
	DB                  *pgx.Conn
	Lang                map[string]interface{}
	CheckHouseCooldowns []string
	CommandsCooldowns   map[string]string
	OtherCooldowns      map[string][]string
	AssignableRoles     map[string]string
}

type House struct {
	Points uint16
	Name   string
	RoleID string
}

func NewHagrid() Hagrid {
	hg := Hagrid{}
	hg.readConfig()
	hg.readLanguages()
	hg.readAssignableRoles()

	rand.Seed(time.Now().UnixNano()) // initialiser rand
	hg.CheckHouseCooldowns = []string{}

	hg.OtherCooldowns = make(map[string][]string)
	hg.OtherCooldowns["intro"] = []string{}

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

func (hg *Hagrid) readAssignableRoles() {
	d, err := os.Open("res/assignable_roles.json")
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	data, _ := ioutil.ReadAll(d)
	_ = json.Unmarshal(data, &hg.AssignableRoles)
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

type Player struct {
	ID          string
	Lang        string
	House       *House
	DatePremium time.Time
	Member      *discordgo.Member
}

func (player *Player) GetLang(search string) string {
	return Hg.GetLang(search, player.Lang)
}

/* Fin Base de données */

var (
	HousesIdenfiers = map[string]*House{
		"GRYFFONDOR": {
			RoleID: "796774549232287754",
			Name:   "Gryffondor",
		},
		"POUFSOUFFLE": {
			RoleID: "796775145317859373",
			Name:   "Poufsouffle",
		},
		"SERPENTARD": {
			RoleID: "796774926383972383",
			Name:   "Serpentard",
		},
		"SERDAIGLE": {
			RoleID: "796775403707826227",
			Name:   "Serdaigle",
		},
	}
	HousesNames = func() []string {
		keys := make([]string, 0, 4)
		for k := range HousesIdenfiers {
			keys = append(keys, k)
		}
		return keys
	}()
)

func (hg *Hagrid) GetHouse(name string, queryDb bool) *House {
	var house *House
	if StringSliceContains(HousesNames, name) {
		house = &House{}
		house = HousesIdenfiers[name]
		if queryDb {
			_ = hg.DB.QueryRow(context.Background(), "SELECT points FROM maisons WHERE nom = $1", name).Scan(&house.Points)
		}
	}
	return house
}

func (hg *Hagrid) GetPlayer(userID string) *Player {
	author, _ := hg.GetMember(userID)
	player := &Player{
		ID:     userID,
		Member: author,
		House:  &House{},
	}

	var prem string
	_ = Hg.DB.QueryRow(context.Background(), `SELECT users.maison, users."datePremium", alluser.lang FROM users INNER JOIN alluser ON users.id = alluser.id WHERE users.id = $1`, userID).Scan(&player.House.Name, prem, &player.Lang)
	if player.Lang == "" {
		player.Lang = "fr"
	}
	premInt, err := strconv.ParseInt(prem, 10, 64)
	if err != nil || premInt == 0 {
		player.DatePremium = time.Unix(0, 0)
	} else {
		player.DatePremium = time.Unix(premInt/1000, 0)
	}
	return player
}

// GetLang : len(lang) != 2 => fetch utilisateur
func (hg *Hagrid) GetLang(search string, lang string) string {
	if len(lang) != 2 {
		lang = hg.GetPlayer(lang).Lang
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
	player := Hg.GetPlayer(userID)
	if player.House.Name != "" { // si il a une maison
		player.House = Hg.GetHouse(player.House.Name, false)
		house := player.House
		for _, h := range HousesIdenfiers {
			if h.RoleID == house.RoleID && !StringSliceContains(memberRoles, house.RoleID) { // si c'est sa maison et qu'il n'a pas le rôle
				_ = hg.Session.GuildMemberRoleAdd(hg.Config.GuildID, userID, h.RoleID)
			} else if h.RoleID != house.RoleID && StringSliceContains(memberRoles, house.RoleID) { // si ce n'est pas sa maison mais qu'il a le rôle
				_ = hg.Session.GuildMemberRoleRemove(hg.Config.GuildID, userID, h.RoleID)
			}
		}
		if player.DatePremium != time.Unix(0, 0) {
			if player.DatePremium.Before(time.Now()) {
				_, _ = Hg.DB.Exec(context.Background(), `UPDATE users SET "datePremium" = '' WHERE id = $1`, player.ID)
				if StringSliceContains(player.Member.Roles, Hg.Config.PremiumRoleID) {
					_ = hg.Session.GuildMemberRoleRemove(hg.Config.GuildID, player.ID, Hg.Config.PremiumRoleID)
				}
			} else {
				if !StringSliceContains(player.Member.Roles, Hg.Config.PremiumRoleID) {
					_ = hg.Session.GuildMemberRoleAdd(hg.Config.GuildID, player.ID, Hg.Config.PremiumRoleID)
				}
			}
		}
	}

	return nil
}
