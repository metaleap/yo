package yocfg

import (
	"bytes"
	"os"
	"reflect"
	"time"

	yoctx "yo/ctx"
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

	YO_MAIL_SMTP_HOST      string
	YO_MAIL_SMTP_PORT      int
	YO_MAIL_SMTP_USERNAME  string
	YO_MAIL_SMTP_PASSWORD  string
	YO_MAIL_SMTP_SENDER    string
	YO_MAIL_SMTP_TIMEOUT   time.Duration
	YO_MAIL_ERR_LOG_FWD_TO string

	YO_DB_CONN_TIMEOUT time.Duration
	YO_DB_CONN_URL     string

	STATIC_FILE_STORAGE_DIRS map[string]string
}

var envFile = str.Dict{}

func init() {
	if IsDevMode && !yoctx.CatchPanics {
		defer func() { // for prolonged debugging/breakpoint staring sessions:
			Cfg.YO_API_IMPL_TIMEOUT = 11 * time.Minute
		}()
	}

	// Setenv from .env file if any
	for _, file_name := range []string{".env", ".env.prod"} {
		if env_file_data := bytes.TrimSpace(ReadFile(file_name)); len(env_file_data) > 0 {
			for i, lines := 0, str.Split(string(env_file_data), "\n"); i < len(lines); i++ {
				if line := str.Trim(lines[i]); line != "" {
					name, val, ok := str.Cut(line, "=")
					if !ok {
						panic(line)
					} else if os.Getenv(name) == "" {
						envFile[name] = val
					}
					for str.Begins(val, "$") {
						val = os.Getenv(val[:1])
					}
				}
			}
		}
		if IsDevMode {
			break
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
			if env_val = envFile[env_name]; env_val == "" {
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

func CfgGet[T any](envVarName string) (ret T) {
	env_val := os.Getenv(envVarName)
	if env_val == "" {
		if env_val = envFile[envVarName]; env_val == "" {
			panic("missing in env: " + envVarName)
		}
	}
	switch any(ret).(type) {
	case string:
		env_val = str.Q(env_val)
	case time.Duration:
		v, err := time.ParseDuration(env_val)
		if err != nil {
			panic(err)
		}
		return any(v).(T)
	}
	yojson.Load([]byte(env_val), &ret)
	return
}
