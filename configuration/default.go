package configuration

func Default() *Configuration {
	return &Configuration{
		Dir:      "data",
		HttpAddr: "127.0.0.1:8080",
	}
}
