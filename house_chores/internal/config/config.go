// Package config picks the run mode from environment variables.
package config

import "os"

type Mode string

const (
	ModeSupervisor Mode = "supervisor"
	ModeFake       Mode = "fake"
)

// IngressGateway is the only address accepted in supervisor mode (Home Assistant ingress).
const IngressGateway = "172.30.32.2"

type Config struct {
	Mode            Mode
	Port            string
	DataDir         string
	SupervisorURL   string // http://supervisor; Core API at /core/api, WebSocket at /core/websocket
	SupervisorToken string
}

// Load: SUPERVISOR_TOKEN set → supervisor mode (port 8099, /data); otherwise fake mode (PORT, DATA_DIR).
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
