package haaukins

import (
	"errors"
	"fmt"
	logger "github.com/rs/zerolog/log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	tagRawRegexp           = `^[a-z0-9][a-z0-9-]*[a-z0-9]$`
	tagRegex               = regexp.MustCompile(tagRawRegexp)
	ErrEmptyTag            = errors.New("Tag cannot be empty")
	ErrUnknownFlag         = errors.New("Unknown flag")
	ErrFlagAlreadyComplete = errors.New("Flag is already completed")
	ErrChallengeDuplicate  = errors.New("Challenge duplication")
)

type Tag string

func NewTag(s string) (Tag, error) {
	t := Tag(s)
	if err := t.Validate(); err != nil {
		return "", err
	}

	return t, nil
}

func (t Tag) Validate() error {
	s := string(t)
	if s == "" {
		return ErrEmptyTag
	}

	if !tagRegex.MatchString(s) {
		return &InvalidTagSyntaxErr{s}
	}

	return nil
}

type Challenge struct {
	Tag  Tag    `json:"tag"`
	Name string `json:"name"`
	Value string `json:"flagValue"`
}

type TeamChallenge struct {
	Tag         Tag        `yaml:"tag"`
	CompletedAt *time.Time `yaml:"completed-at,omitempty"`
}

type Team struct {
	m sync.RWMutex

	id             string
	email          string
	name           string
	hashedPassword string
	challenges     map[Flag]TeamChallenge
	registeredAt   *time.Time
}

func NewTeam(email, name, password, id, hashedPass string) *Team {
	now := time.Now()
	var hPass []byte
	if hashedPass==""{
		hPass ,_ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	} else {
		hPass = []byte(hashedPass)
	}

	email = strings.ToLower(email)
	if id == "" {
		id =  uuid.New().String()[0:8]
	}

	return &Team{
		id:             id,
		email:          email,
		name:           name,
		hashedPassword: string(hPass),
		challenges:     map[Flag]TeamChallenge{},
		registeredAt:   &now,
	}
}

func (t *Team) ID() string {
	t.m.RLock()
	id := t.id
	t.m.RUnlock()

	return id
}

func (t *Team) Email() string {
	t.m.RLock()
	email := t.email
	t.m.RUnlock()

	return email
}


func(t *Team) GetHashedPassword() string{
	t.m.RLock()
	defer t.m.RUnlock()
	return t.hashedPassword
}

func (t *Team) Name() string {
	t.m.RLock()
	name := t.name
	t.m.RUnlock()

	return name
}

func (t *Team) IsTeamSolvedChallenge(tag string) *time.Time {
	chals := t.challenges
	for _, chal := range chals {
		if chal.Tag == Tag(tag) {
			if chal.CompletedAt != nil {
				return chal.CompletedAt
			}
		}
	}
	return nil
}

func (t *Team) IsPasswordEqual(pass string) bool {
	t.m.RLock()
	err := bcrypt.CompareHashAndPassword([]byte(t.hashedPassword), []byte(pass))
	t.m.RUnlock()
	return err == nil
}

func (t *Team) AddChallenge(c Challenge) (Flag, error) {
	t.m.Lock()
	for _, chal := range t.challenges {
		if chal.Tag == c.Tag {
			t.m.Unlock()
			return Flag{}, ErrChallengeDuplicate
		}
	}

	f , err := NewFlagFromString(c.Value)
	if err !=nil {
		logger.Debug().Msgf("Error creating haaukins flag from given string %s", err)
		return Flag{}, err
	}
	t.challenges[f] = TeamChallenge{
		Tag: c.Tag,
	}

	t.m.Unlock()
	return f, nil
}

func (t *Team) GetChallenges(order ...Tag) []TeamChallenge {
	t.m.RLock()
	var chals []TeamChallenge
	if len(order) > 0 {
	loop:
		for _, tag := range order {
			for _, chal := range t.challenges {
				if tag == chal.Tag {
					chals = append(chals, chal)
					continue loop
				}
			}
		}
		t.m.RUnlock()
		return chals
	}

	for _, chal := range t.challenges {
		chals = append(chals, chal)
	}

	t.m.RUnlock()
	return chals
}

func (t *Team) VerifyFlag(tag Challenge, f Flag) error {
	t.m.Lock()
	chal, ok := t.challenges[f]

	if !ok {
		t.m.Unlock()
		return ErrUnknownFlag
	}

	fmt.Println(string(chal.Tag)+" ... "+string(tag.Tag))
	if chal.Tag != tag.Tag{
		t.m.Unlock()
		return ErrUnknownFlag
	}

	if chal.CompletedAt != nil {
		t.m.Unlock()
		return ErrFlagAlreadyComplete
	}
	now := time.Now()
	chal.CompletedAt = &now
	t.challenges[f] = chal

	t.m.Unlock()
	return nil
}

func (es *Team) GetTeams() []Team {
	var teams []Team
	for _, t := range es.GetTeams() {
		teams = append(teams, t)
	}

	return teams
}

type InvalidTagSyntaxErr struct {
	tag string
}

func (ite *InvalidTagSyntaxErr) Error() string {
	return fmt.Sprintf("Invalid syntax for tag \"%s\", allowed syntax: %s", ite.tag, tagRawRegexp)
}

func (t *Team)SolvedChallenges() map[Flag]TeamChallenge {
	return t.challenges
}

func (t *Team) SolveChallenge(flag Flag,tag Tag, time *time.Time)  {
	// not returning something is nott good approach however it is added for testing purposes for now !
	t.challenges[flag]=TeamChallenge{Tag:tag, CompletedAt:time}
}