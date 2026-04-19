package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

type App struct {
	DB      *sql.DB
	Tmpl    *template.Template
	AppName string
	DataDir string

	dailyPIN string
	pinDate  string
	mu       sync.Mutex
}

func (a *App) EnsureDailyPIN() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if a.pinDate != today {
		a.pinDate = today
		a.dailyPIN = fmt.Sprintf("%04d", rand.Intn(10000))
		log.Printf("New daily PIN for %s: %s", today, a.dailyPIN)
	}
	return a.dailyPIN
}

// SetPIN sets a known PIN for testing.
func (a *App) SetPIN(pin string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dailyPIN = pin
	a.pinDate = time.Now().Format("2006-01-02")
}

func GetLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func GetMDNSHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	hostname = strings.TrimSuffix(hostname, ".local")
	hostname = strings.TrimSuffix(hostname, ".")
	return strings.ToLower(hostname) + ".local"
}

func GenerateQR(content string) string {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		log.Printf("QR generation failed: %v", err)
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func NoCache(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
