package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-rod/rod/lib/launcher"
)

// chromeBin resolves the browser binary for rod.
// Order: CHROME_PATH / WECHAT_PEN_CHROME → common Linux paths → Windows Chrome → launcher default.
func chromeBin() string {
	for _, k := range []string{"CHROME_PATH", "WECHAT_PEN_CHROME"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			if _, err := os.Stat(v); err == nil {
				return v
			}
		}
	}
	if runtime.GOOS == "windows" {
		p := `C:\Program Files\Google\Chrome\Application\chrome.exe`
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return ""
	}
	candidates := []string{
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/google-chrome",
		"/snap/bin/chromium",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("chromium"); err == nil {
		return p
	}
	if p, err := exec.LookPath("google-chrome"); err == nil {
		return p
	}
	return ""
}

// wantHeadless: Docker/CI usually has no DISPLAY. Override with WECHAT_PEN_HEADLESS=0|1.
func wantHeadless() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("WECHAT_PEN_HEADLESS"))) {
	case "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	}
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" {
		return true
	}
	return false
}

// inContainer is a best-effort check (Docker/K8s).
func inContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if b, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(b)
		return strings.Contains(s, "docker") || strings.Contains(s, "containerd") || strings.Contains(s, "kubepods")
	}
	return false
}

// launchChrome starts Chromium/Chrome via rod launcher and returns the control URL.
// profileDir empty → ephemeral profile.
func launchChrome(profileDir string) (controlURL string, err error) {
	if profileDir != "" {
		if err := os.MkdirAll(profileDir, 0o700); err != nil {
			return "", fmt.Errorf("profile dir: %w", err)
		}
		// stale singleton lock often blocks relaunch in containers
		_ = os.Remove(filepath.Join(profileDir, "SingletonLock"))
		_ = os.Remove(filepath.Join(profileDir, "SingletonCookie"))
		_ = os.Remove(filepath.Join(profileDir, "SingletonSocket"))
	}

	l := launcher.New().Headless(wantHeadless())
	if profileDir != "" {
		l = l.UserDataDir(profileDir)
	}
	if bin := chromeBin(); bin != "" {
		l = l.Bin(bin)
	}

	// Required for Chromium in Docker / as rootless restricted envs
	if inContainer() || wantHeadless() || runtime.GOOS == "linux" {
		l = l.
			Set("no-sandbox").
			Set("disable-gpu").
			Set("disable-dev-shm-usage").
			Set("disable-software-rasterizer")
	}

	u, err := l.Launch()
	if err != nil {
		return "", err
	}
	return u, nil
}
