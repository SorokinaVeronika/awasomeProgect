package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/sirupsen/logrus"

	"awesomeProject/models"
)

type Server struct {
	logger *logrus.Logger
	store  *Database
}

func NewServer(logger *logrus.Logger, store *Database) *Server {
	return &Server{
		logger: logger,
		store:  store,
	}
}

func (s Server) GetAllTickers() ([]string, error) {
	return s.store.GetAllIDs()
}

func (s Server) GetETF(ticker string) (*models.ETFData, error) {
	etf, err := s.store.GetByID(ticker)
	if err != nil {
		return nil, err
	}

	var data models.ETFData
	err = json.Unmarshal(etf.Data, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func (s Server) UserExists(username, password string) (bool, error) {
	return s.store.UserExists(username, toHash(password))
}

func toHash(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	hashBytes := hasher.Sum(nil)

	return hex.EncodeToString(hashBytes)
}
