package configuration

type Configuration struct {
	HttpAddr string `usage:"HTTP address"`
	Dir      string `usage:"data directory"`
}
