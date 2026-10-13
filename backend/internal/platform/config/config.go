package config

import (
	"errors"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL                                                                      string
	DBMaxOpenConns, DBMaxIdleConns                                                   int
	DBConnectTimeout, DBQueryTimeout, DBConnMaxLifetime, DBConnMaxIdleTime           time.Duration
	DataDir, Addr, Version, FFmpeg, FFprobe, DatasetAPIURL, DatasetAPIKey, DatasetID string
	MaxUploadBytes                                                                   int64
	QueueSize, MaxFrames                                                             int
	JobTimeout, HTTPTimeout                                                          time.Duration
	AllowedPrivateOrigins, AllowedMediaOrigins                                       []string
}

func Load(get func(string) string) (Config, error) {
	c := Config{DataDir: get("APP_DATA_DIR"), Addr: value(get, "APP_ADDR", ":8080"), Version: value(get, "APP_VERSION", "dev"), FFmpeg: value(get, "APP_FFMPEG", "ffmpeg"), FFprobe: value(get, "APP_FFPROBE", "ffprobe"), DatasetAPIURL: get("APP_DATASET_API_URL"), DatasetAPIKey: get("APP_DATASET_API_KEY")}
	c.DatabaseURL = get("APP_DATABASE_URL")
	u, databaseErr := url.Parse(c.DatabaseURL)
	if databaseErr != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return c, errors.New("APP_DATABASE_URL must be a PostgreSQL URL")
	}
	for _, option := range []struct {
		key string
		def int64
		dst *int
	}{{"APP_DB_MAX_OPEN_CONNS", 5, &c.DBMaxOpenConns}, {"APP_DB_MAX_IDLE_CONNS", 2, &c.DBMaxIdleConns}} {
		n, err := positive(get, option.key, "", option.def)
		if err != nil || n > 5 {
			return c, errors.New("invalid database pool size")
		}
		*option.dst = int(n)
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		return c, errors.New("database idle connections exceed open connections")
	}
	for _, option := range []struct {
		key string
		def int64
		dst *time.Duration
	}{{"APP_DB_CONNECT_TIMEOUT_SECONDS", 5, &c.DBConnectTimeout}, {"APP_DB_QUERY_TIMEOUT_SECONDS", 10, &c.DBQueryTimeout}, {"APP_DB_CONN_MAX_LIFETIME_SECONDS", 1800, &c.DBConnMaxLifetime}, {"APP_DB_CONN_MAX_IDLE_SECONDS", 300, &c.DBConnMaxIdleTime}} {
		n, err := positive(get, option.key, "", option.def)
		if err != nil || n > 86400 {
			return c, errors.New("invalid database timeout")
		}
		*option.dst = time.Duration(n) * time.Second
	}
	c.DatasetID = get("APP_DATASET_ID")
	_, port, addrErr := net.SplitHostPort(c.Addr)
	portNumber, portErr := strconv.ParseUint(port, 10, 16)
	if addrErr != nil || portErr != nil || portNumber == 0 {
		return c, errors.New("APP_ADDR must include a valid port")
	}
	if len(c.DatasetID) > 128 || strings.ContainsAny(c.DatasetID, "/\\?#") {
		return c, errors.New("invalid dataset ID")
	}
	if c.DataDir == "" {
		c.DataDir = get("DATA_DIR")
	}
	if !filepath.IsAbs(c.DataDir) {
		return c, errors.New("APP_DATA_DIR must be absolute")
	}
	c.DataDir = filepath.Clean(c.DataDir)
	if insideCheckout(c.DataDir) {
		return c, errors.New("APP_DATA_DIR must be outside a source checkout")
	}
	n, err := positive(get, "APP_MAX_UPLOAD_BYTES", "MAX_UPLOAD_BYTES", 1073741824)
	if err != nil || n > math.MaxInt64-1024*1024-1 {
		if err == nil {
			err = errors.New("APP_MAX_UPLOAD_BYTES exceeds supported range")
		}
		return c, err
	}
	c.MaxUploadBytes = n
	n, err = positive(get, "APP_QUEUE_SIZE", "", 8)
	if err != nil || n > 100 {
		return c, errors.New("APP_QUEUE_SIZE must be 1..100")
	}
	c.QueueSize = int(n)
	n, err = positive(get, "APP_MAX_FRAMES", "", 1000)
	if err != nil || n > 100000 {
		return c, errors.New("APP_MAX_FRAMES must be 1..100000")
	}
	c.MaxFrames = int(n)
	n, err = positive(get, "APP_JOB_TIMEOUT_SECONDS", "", 600)
	if err != nil || n > 86400 {
		return c, errors.New("invalid job timeout")
	}
	c.JobTimeout = time.Duration(n) * time.Second
	n, err = positive(get, "APP_HTTP_TIMEOUT_SECONDS", "", 30)
	if err != nil || n > 3600 {
		return c, errors.New("invalid HTTP timeout")
	}
	c.HTTPTimeout = time.Duration(n) * time.Second
	for _, pair := range []struct {
		key string
		dst *[]string
	}{{"APP_ALLOWED_PRIVATE_ORIGINS", &c.AllowedPrivateOrigins}, {"APP_ALLOWED_MEDIA_ORIGINS", &c.AllowedMediaOrigins}} {
		if v := get(pair.key); v != "" {
			for _, origin := range strings.Split(v, ",") {
				u, e := url.Parse(strings.TrimSpace(origin))
				if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
					return c, errors.New("invalid allowed origin")
				}
				*pair.dst = append(*pair.dst, u.Scheme+"://"+u.Host)
			}
		}
	}
	if c.DatasetAPIURL != "" {
		u, e := url.Parse(c.DatasetAPIURL)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return c, errors.New("invalid dataset API URL")
		}
	}
	return c, nil
}
func insideCheckout(path string) bool {
	for candidate := path; ; candidate = filepath.Dir(candidate) {
		resolved, e := filepath.EvalSymlinks(candidate)
		if e == nil {
			for p := resolved; ; p = filepath.Dir(p) {
				if _, e := os.Stat(filepath.Join(p, ".git")); e == nil {
					return true
				}
				if filepath.Dir(p) == p {
					break
				}
			}
		}
		if _, e := os.Stat(filepath.Join(candidate, ".git")); e == nil {
			return true
		}
		if filepath.Dir(candidate) == candidate {
			return false
		}
	}
}
func value(get func(string) string, key, def string) string {
	if v := get(key); v != "" {
		return v
	}
	return def
}
func positive(get func(string) string, key, alias string, def int64) (int64, error) {
	s := get(key)
	if s == "" && alias != "" {
		s = get(alias)
	}
	if s == "" {
		return def, nil
	}
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n <= 0 {
		return 0, errors.New(key + " must be positive")
	}
	return n, nil
}
