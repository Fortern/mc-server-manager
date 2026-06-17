package minecraft

import (
	"os"
	"time"

	"github.com/google/uuid"
)

// Server is a Minecraft server
type Server struct {
	// Server's Name
	Name string
	// Server's main Domain
	Domain string
	// Server's QQ group number
	QGroupNum uint64
	//
	CoreSubServer *SubServer
}

type SubServer struct {
	ID int `grom:"primary"`
	// SubServer's Name
	Name string
	// SubServer's Path
	Path os.File
	//
	Type SrvType
}

// Player is a server player
type Player struct {
	// Name is player's nickname.
	Name string
	// QNum is player's QQ number.
	QNum uint64
	// FirstJoinTime is the time when the player first joined.
	FirstJoinTime time.Time
	// Present If true, the player is on this server; otherwise, they have left.
	Present bool
	// LastLeaveTime is the time when the player last left.
	LastLeaveTime time.Time
	// WhiteListed if true, the player is on the whitelist.
	WhiteListed bool
	// Players' Main Account
	McAccount
	// Players' Secondary Accounts
	SecondaryAccounts []*McAccount
}

// McAccount is a Minecraft Account
type McAccount struct {
	// The UUID of a player's Minecraft profile
	UUID uuid.UUID
	// The McName of a player's Minecraft profile
	McName string
}

type SrvType int

const (
	Velocity SrvType = iota
	Minecraft
	MCDR
)
