package mediaagent

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func readSecret(path string) (string, error) {
	if !filepath.IsAbs(path) || !outsideCheckout(path) {
		return "", errors.New("INVALID_CONFIG")
	}
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 8192 {
		return "", errors.New("INVALID_CONFIG")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return "", errors.New("INVALID_CONFIG")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", errors.New("INVALID_CONFIG")
	}
	secret := strings.TrimSpace(string(b))
	if len(secret) < 16 || strings.ContainsAny(secret, "\r\n\t ") {
		return "", errors.New("INVALID_CONFIG")
	}
	return secret, nil
}
func outsideCheckout(path string) bool {
	cwd, e := os.Getwd()
	if e != nil {
		return false
	}
	for d := cwd; ; d = filepath.Dir(d) {
		if _, e = os.Stat(filepath.Join(d, ".git")); e == nil {
			if !strings.EqualFold(filepath.VolumeName(d), filepath.VolumeName(path)) {
				return true
			}
			rel, e := filepath.Rel(d, path)
			return e == nil && (rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)))
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	return true
}
func numberEnv(key string, defaultValue int64) (int64, error) {
	if s := os.Getenv(key); s != "" {
		n, e := strconv.ParseInt(s, 10, 64)
		if e != nil || n <= 0 {
			return 0, errors.New("INVALID_CONFIG")
		}
		return n, nil
	}
	return defaultValue, nil
}
func LoadConfig() (Config, string, error) {
	addr := os.Getenv("MAC_AGENT_ADDR")
	if addr == "" {
		addr = "127.0.0.1:19092"
	}
	host, port, e := net.SplitHostPort(addr)
	ip := net.ParseIP(host)
	n, e2 := strconv.Atoi(port)
	if e != nil || e2 != nil || ip == nil || !ip.IsLoopback() || n <= 0 || n > 65535 || n == 19090 {
		return Config{}, "", errors.New("INVALID_CONFIG")
	}
	c := Config{Root: os.Getenv("MAC_MEDIA_ROOT"), FFmpeg: os.Getenv("MAC_FFMPEG"), FFprobe: os.Getenv("MAC_FFPROBE"), SourceURL: os.Getenv("MAC_SOURCE_SERVICE_URL")}
	for _, tool := range []string{c.FFmpeg, c.FFprobe} {
		if tool != "" && !filepath.IsAbs(tool) {
			return Config{}, "", errors.New("INVALID_CONFIG")
		}
	}
	if !filepath.IsAbs(c.Root) || !outsideCheckout(c.Root) {
		return Config{}, "", errors.New("INVALID_CONFIG")
	}
	c.Token, e = readSecret(os.Getenv("MAC_TOKEN_FILE"))
	if e != nil {
		return Config{}, "", e
	}
	if sourceTokenFile := os.Getenv("MAC_SOURCE_TOKEN_FILE"); sourceTokenFile != "" {
		c.SourceToken, e = readSecret(sourceTokenFile)
		if e != nil {
			return Config{}, "", e
		}
		if c.SourceURL == "" {
			c.SourceURL = "http://127.0.0.1:19090"
		}
	} else if c.SourceURL != "" {
		return Config{}, "", errors.New("INVALID_CONFIG")
	}
	for _, item := range []struct {
		key      string
		fallback int64
		target   *int64
	}{{"MAC_MAX_UPLOAD_BYTES", 1 << 30, &c.MaxUploadBytes}, {"MAC_MAX_CONTROL_BYTES", 16 << 20, &c.MaxControlBytes}, {"MAC_MAX_ARCHIVE_IMAGE_BYTES", 2 << 30, &c.MaxArchiveImageBytes}} {
		*item.target, e = numberEnv(item.key, item.fallback)
		if e != nil {
			return Config{}, "", e
		}
	}
	for _, item := range []struct {
		key      string
		fallback int64
		target   *int
	}{{"MAC_MAX_FRAMES", 1000, &c.MaxFrames}, {"MAC_QUEUE_SIZE", 8, &c.QueueSize}} {
		v, err := numberEnv(item.key, item.fallback)
		if err != nil || v > 10000 {
			return Config{}, "", errors.New("INVALID_CONFIG")
		}
		*item.target = int(v)
	}
	for _, item := range []struct {
		key      string
		fallback int64
		target   *time.Duration
	}{{"MAC_OPERATION_LEASE_SECONDS", 120, &c.Lease}, {"MAC_MAX_OPERATION_SECONDS", 600, &c.MaxOperation}, {"MAC_ARCHIVE_TTL_SECONDS", 3600, &c.ArchiveTTL}} {
		v, err := numberEnv(item.key, item.fallback)
		if err != nil || v > 86400 {
			return Config{}, "", errors.New("INVALID_CONFIG")
		}
		*item.target = time.Duration(v) * time.Second
	}
	if c.MaxOperation > 600*time.Second {
		return Config{}, "", errors.New("INVALID_CONFIG")
	}
	base, id, keyFile := os.Getenv("MAC_DATASET_API_URL"), os.Getenv("MAC_DATASET_ID"), os.Getenv("MAC_DATASET_API_KEY_FILE")
	if base != "" || id != "" || keyFile != "" {
		if base == "" || id == "" || keyFile == "" {
			return Config{}, "", errors.New("INVALID_CONFIG")
		}
		key, err := readSecret(keyFile)
		if err != nil {
			return Config{}, "", err
		}
		c.Dataset = NewDatasetAdapter(base, id, key, splitOrigins(os.Getenv("MAC_DATASET_ALLOWED_PRIVATE_ORIGINS")), splitOrigins(os.Getenv("MAC_DATASET_ALLOWED_MEDIA_ORIGINS")))
	}
	return c, addr, nil
}
func splitOrigins(s string) []string {
	var values []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			values = append(values, v)
		}
	}
	return values
}
