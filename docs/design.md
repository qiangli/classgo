Excellent choice. **ClassGo** is perfectly punny for a Golang project and conveys exactly what the app does—gets students through the door and into the lesson.

---

# Plan: ClassGo (Private Tutoring Attendance)

## 1. Project Overview
**ClassGo** is a lightweight, local-network attendance server built in Go. It is designed for private tutors to track small groups of students. The server runs on the tutor's laptop, and students sign in via a shared tablet or their own mobile devices over local Wi-Fi.

## 2. Core Features
* **Local-First:** Works without an internet connection (great for privacy and reliability).
* **Dual Entry Modes:**
    * **Mobile Mode:** Students scan a QR code and sign in on their own phones.
    * **Kiosk Mode:** A simplified, high-speed interface for a stationary classroom tablet.
* **Session Security:** A "Daily Pin" ensures students are physically present in the room.
* **Data Export:** Generates CSV files for easy billing and student progress tracking.

---

## 3. Technical Stack
* **Backend:** Golang (using `net/http` for the server and `html/template` for UI).
* **Database:** SQLite (a single-file database that lives in the project folder).
* **Frontend:** HTML5, Tailwind CSS (via CDN), and Vanilla JavaScript.
* **Network:** Local IP binding (Host laptop acts as the hub).

---

## 4. Implementation Phases

### Phase 1: Database & Models
* [ ] Define `Attendance` struct: `ID`, `StudentID`, `StudentName`, `DeviceType`, `Timestamp`.
* [ ] Set up SQLite initialization script to create the `attendance` table if it doesn't exist.

### Phase 2: The Go Server Core
* [ ] Create a router with the following endpoints:
    * `GET /`: The mobile sign-in landing page.
    * `GET /kiosk`: The tablet-optimized sign-in page.
    * `GET /admin`: The tutor's dashboard (list of attendees + QR code).
    * `POST /api/signin`: The logic to validate the PIN and save to the DB.
    * `GET /admin/export`: Generates and downloads a `.csv` file.

### Phase 3: Frontend "ClassGo" UI
* **Mobile View:** Includes a "Remember Me" checkbox using `localStorage` so repeat students only have to enter their name once.
* **Tablet (Kiosk) View:** * Features a large numeric input.
    * **Auto-Reset:** After clicking "Sign In," the page shows a success message for 3 seconds, then resets to blank for the next student in line.

### Phase 4: Network Connectivity
* [ ] Implement a function to auto-detect the laptop's Local IP address.
* [ ] Display a generated QR code on the `/admin` page so students can scan it instantly.

---

## 5. Directory Structure
```text
/ClassGo
├── main.go             # All Go logic (Server, DB, Routes)
├── classgo.db          # SQLite Database file
├── /templates          # HTML Views
│   ├── mobile.html     # Optimized for personal phones
│   ├── kiosk.html      # Optimized for shared tablets
│   └── admin.html      # Tutor control panel
└── /static             # CSS and Branding images
```

---

## 6. Success Metrics for Tutoring
* **Speed:** A student should be able to sign in on the tablet in under 5 seconds.
* **Accuracy:** The admin panel should update in real-time as students arrive.
* **Portability:** The tutor can move to different locations (cafes, homes, studios) and the app will adapt to the new local Wi-Fi.

---

**Would you like me to start by writing the `main.go` file with the SQLite connection and the basic server setup?**