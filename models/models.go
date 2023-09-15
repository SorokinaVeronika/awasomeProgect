package models

import (
	"encoding/json"
	"github.com/dgrijalva/jwt-go"
	"time"
)

type ETF struct {
	ID        string
	Data      []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	ID        int
	Username  string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ETFData struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	TopHoldings []Holding    `json:"top_holdings"`
	Countries   []WeightData `json:"countries"`
	Sectors     []WeightData `json:"sectors,omitempty"`
}

func (m ETFData) ToJson() []byte {
	res, _ := json.Marshal(m)
	return res
}

type Holding struct {
	Name       string `json:"name"`
	SharesHeld string `json:"shares_held"`
	Weight     string `json:"weight"`
}

type WeightData struct {
	Name   string `json:"name"`
	Weight string `json:"weight"`
}

type GeographicalData struct {
	AttributeArray []CountryWeight
}

type CountryWeight struct {
	Name struct {
		Value string `json:"value"`
	} `json:"name"`
	Weight struct {
		Value         string `json:"value"`
		OriginalValue string `json:"originalValue"`
	} `json:"weight"`
}

type SectorWeight struct {
	Name   string
	Weight string
}

type FundHoldings struct {
	HoldingName string
	SharesHeld  string
	Weight      string
}

// Claims - Define a struct for JWT claims
type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}
