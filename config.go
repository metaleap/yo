package yo

import (
	"os"
	"reflect"
	"strconv"
	"time"
)

var cfg config

type config struct {
	YO_API_HTTP_PORT    int
	YO_API_IMPL_TIMEOUT time.Duration
}

func cfgLoad(prefix string) {
	cfg := reflect.ValueOf(cfg)
	config := cfg.Type()
	for i := 0; i < config.NumField(); i++ {
		env_name := config.Field(i).Name
		env_val := os.Getenv(env_name)
		var new_val any
		switch t := cfg.Field(i).Interface().(type) {
		case int:
			v, err := strconv.ParseInt(env_val, 0, 64)
			if err != nil {
				panic(err)
			}
			new_val = int(v)
		case time.Duration:
			v, err := time.ParseDuration(env_val)
			if err != nil {
				panic(err)
			}
			new_val = v
		default:
			panic(t)
		}
		cfg.Field(i).Set(reflect.ValueOf(new_val))
	}
}
