package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"classgo/internal/database"
	"classgo/internal/models"
)

func (a *App) HandleSchedulePage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin#schedule", http.StatusFound)
}

// Keep old /schedule path working — redirect to new location.
func (a *App) HandleScheduleRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/schedule", http.StatusMovedPermanently)
}

// HandleHome serves the Safari-style start page for regular users.
// If not authenticated, falls back to showing the login form.
func (a *App) HandleHome(w http.ResponseWriter, r *http.Request) {
	sess := a.GetSession(r)
	if sess == nil {
		// Not logged in — show the entry page with login form
		ipURL := fmt.Sprintf("http://%s:8080", GetLocalIP())
		mdnsURL := fmt.Sprintf("http://%s:8080", GetMDNSHostname())
		data := models.CheckInPageData{
			AppName:       a.AppName,
			ServerURLIP:   ipURL,
			ServerURLMDNS: mdnsURL,
		}
		a.Tmpl.ExecuteTemplate(w, "entry.html", data)
		return
	}
	if sess.Role == "admin" {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	name, _ := a.lookupEntity(sess.EntityID)
	if name == "" {
		name = sess.Username
	}

	data := models.DashboardData{
		AppName:  a.AppName,
		UserType: sess.UserType,
		EntityID: sess.EntityID,
		UserName: name,
		Date:     time.Now().Format("Monday, January 2, 2006"),
		Accounts: a.GetAccountInfo(r),
	}
	a.Tmpl.ExecuteTemplate(w, "home.html", data)
}

func (a *App) HandleMobile(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	ipURL := fmt.Sprintf("http://%s:8080", GetLocalIP())
	mdnsURL := fmt.Sprintf("http://%s:8080", GetMDNSHostname())
	data := models.CheckInPageData{
		AppName:       a.AppName,
		ServerURLIP:   ipURL,
		ServerURLMDNS: mdnsURL,
	}
	a.Tmpl.ExecuteTemplate(w, "entry.html", data)
}

func (a *App) HandleKiosk(w http.ResponseWriter, r *http.Request) {
	ipURL := fmt.Sprintf("http://%s:8080", GetLocalIP())
	mdnsURL := fmt.Sprintf("http://%s:8080", GetMDNSHostname())
	data := models.CheckInPageData{
		AppName:       a.AppName,
		QRDataURIIP:   template.URL(GenerateQR(ipURL)),
		QRDataURIMDNS: template.URL(GenerateQR(mdnsURL)),
		ServerURLIP:   ipURL,
		ServerURLMDNS: mdnsURL,
	}
	a.Tmpl.ExecuteTemplate(w, "kiosk.html", data)
}

func (a *App) HandleDirectory(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin#data", http.StatusFound)
}

func (a *App) HandleAdmin(w http.ResponseWriter, r *http.Request) {
	pin := a.EnsureDailyPIN()
	ipURL := fmt.Sprintf("http://%s:8080", GetLocalIP())
	mdnsURL := fmt.Sprintf("http://%s:8080", GetMDNSHostname())

	attendees, err := database.TodayAttendees(a.DB)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Printf("Error fetching attendees: %v", err)
		return
	}

	sess := a.GetSession(r)

	data := models.AdminData{
		AppName:       a.AppName,
		PIN:           pin,
		RequirePIN:    a.RequirePIN(),
		QRDataURIIP:   template.URL(GenerateQR(ipURL)),
		QRDataURIMDNS: template.URL(GenerateQR(mdnsURL)),
		ServerURLIP:   ipURL,
		ServerURLMDNS: mdnsURL,
		Attendees:     attendees,
		Count:         len(attendees),
		Date:          time.Now().Format("Monday, January 2, 2006"),
		IsSuperAdmin:  sess != nil && sess.IsSuperAdmin,
		Accounts:      a.GetAccountInfo(r),
	}
	a.Tmpl.ExecuteTemplate(w, "admin.html", data)
}
