package yo

import (
	"bytes"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var cfg config

type config struct {
	YO_API_HTTP_PORT    int
	YO_API_IMPL_TIMEOUT time.Duration
}

func cfgLoad() {
	// Setenv from .env file if any
	env_file_data, err := os.ReadFile(".env")
	if err != nil {
		panic(err)
	}
	if env_file_data = bytes.TrimSpace(env_file_data); len(env_file_data) == 0 {
		panic(".env")
	} else {
		for i, lines := 0, strings.Split(string(env_file_data), "\n"); i < len(lines); i++ {
			if name, val, ok := strings.Cut(lines[i], "="); !ok {
				panic(lines[i])
			} else if err := os.Setenv(name, val); err != nil {
				panic(err)
			}
		}
	}

	// fill fields in cfg
	ptr := reflect.ValueOf(&cfg)
	struc := ptr.Elem()
	tstruc := ptr.Type().Elem()
	for i := 0; i < tstruc.NumField(); i++ {
		env_name := tstruc.Field(i).Name
		env_val := os.Getenv(env_name)
		var new_val any
		switch t := struc.Field(i).Interface().(type) {
		case string:
			new_val = env_val
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
		struc.Field(i).Set(reflect.ValueOf(new_val))
	}
}
