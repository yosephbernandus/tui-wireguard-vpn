package vpn

import "time"

type Environment string

const (
	Production    Environment = "prod"
	NonProduction Environment = "nonprod"
)

type ConnectionStatus struct {
	Connected   bool
	Environment Environment
	Interface   string
	Endpoint    string
	LastSeen    *time.Time
	BytesRx     uint64
	BytesTx     uint64
}

type Service interface {
	GetStatus() (*ConnectionStatus, error)
	Start(env Environment) error
	Stop() error
	UpdateConfig(userConfigPath string) error
	GetConfig(env Environment) (string, error)
}