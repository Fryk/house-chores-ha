// Package config wybiera tryb pracy appki na podstawie zmiennych środowiskowych.
package config

import "os"

type Mode string

const (
	ModeSupervisor Mode = "supervisor"
	ModeFake       Mode = "fake"
)

// IngressGateway to jedyny adres, z którego w trybie supervisor przyjmujemy połączenia.
const IngressGateway = "172.30.32.2"

type Config struct {
	Mode            Mode
	Port            string
	DataDir         string
	SupervisorURL   string // http://supervisor; Core API pod /core/api, WebSocket pod /core/websocket
	SupervisorToken string
}

// Load: z SUPERVISOR_TOKEN → tryb supervisor (port 8099, /data), bez niego → tryb fake (PORT, DATA_DIR).
func Load() Config {
	if tok := os.Getenv("SUPERVISOR_TOKEN"); tok != "" {
		return Config{
			Mode:            ModeSupervisor,
			Port:            "8099",
			DataDir:         "/data",
			SupervisorURL:   "http://supervisor",
			SupervisorToken: tok,
		}
	}
	return Config{
		Mode:    ModeFake,
		Port:    getenv("PORT", "8080"),
		DataDir: getenv("DATA_DIR", "./data"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
