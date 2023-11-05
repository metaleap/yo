package yocfg

import (
	"bytes"
	"os"
	"reflect"
	"time"

	yojson "yo/json"
	. "yo/util"
	"yo/util/str"
)

var Cfg struct {
	YO_API_HTTP_PORT                int
	YO_API_IMPL_TIMEOUT             time.Duration
	YO_API_MAX_REQ_CONTENTLENGTH_MB int
	YO_API_ADMIN_USER               string
	YO_API_ADMIN_PWD                string

	YO_AUTH_JWT_EXPIRY_DAYS int
	YO_AUTH_JWT_SIGN_KEY    string
	YO_AUTH_PWD_MIN_LEN     int
	YO_AUTH_PWD_MAX_LEN     int

	YO_MAIL_SMTP_HOST     string
	YO_MAIL_SMTP_PORT     int
	YO_MAIL_SMTP_USERNAME string
	YO_MAIL_SMTP_PASSWORD string
	YO_MAIL_SMTP_SENDER   string
	YO_MAIL_SMTP_TIMEOUT  time.Duration

	DB_REQ_TIMEOUT time.Duration
	DATABASE_URL   string

	STATIC_FILE_STORAGE_DIRS map[string]string
}

func init() {
	if IsDevMode {
		defer func() { // for prolonged debugging/breakpoint staring sessions:
			Cfg.DB_REQ_TIMEOUT = time.Minute
			Cfg.YO_API_IMPL_TIMEOUT = 22 * time.Minute
		}()
	}

	env_file := str.Dict{}
	// Setenv from .env file if any
	if env_file_data := bytes.TrimSpace(ReadFile(".env")); len(env_file_data) > 0 {
		for i, lines := 0, str.Split(string(env_file_data), "\n"); i < len(lines); i++ {
			if name, val, ok := str.Cut(lines[i], "="); !ok {
				panic(lines[i])
			} else if os.Getenv(name) == "" {
				env_file[name] = val
			}
		}
	}

	// fill fields in cfg
	ptr := reflect.ValueOf(&Cfg)
	struc := ptr.Elem()
	tstruc := ptr.Type().Elem()
	for i := 0; i < tstruc.NumField(); i++ {
		env_name := tstruc.Field(i).Name
		env_val := os.Getenv(env_name)
		if env_val == "" {
			if env_val = env_file[env_name]; env_val == "" {
				panic("missing in env: " + env_name)
			}
		}
		var new_val any
		switch struc.Field(i).Interface().(type) {
		case string:
			new_val = env_val
		case int:
			v, err := str.ToI64(env_val, 0, 64)
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
			ptr := reflect.New(struc.Field(i).Type()).Interface()
			yojson.Load([]byte(env_val), ptr)
			new_val = reflect.ValueOf(ptr).Elem().Interface()
		}
		struc.Field(i).Set(reflect.ValueOf(new_val))
	}
}
