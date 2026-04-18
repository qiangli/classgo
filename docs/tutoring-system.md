Based on our discussion and the existing [ClassGo](https://github.com/qiangli/classgo) codebase, here is the summarized development plan for your tutoring management system.

---

## 1. Core Architecture: "The LERN Stack"
The app will follow a **Local-First** philosophy where the filesystem is the source of truth, and [Go](https://go.dev/) manages the logic.

* **Primary Backend:** Go (using `net/http` and `html/template`).
* **Database (High-Speed Index):** [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (Pure Go, no CGO) to index filesystem data for fast UI queries.
* **Source of Truth:** A local `./data` directory containing human-readable JSON files.
* **Frontend Shell:** [Memos](https://github.com/usememos/memos) for communication, student notes, and the primary user timeline.

---

## 2. Storage & Backup Strategy
You will implement a **One-Way Mirror** to [Google Drive](https://www.google.com/drive/) to ensure data safety without the complexity of a full cloud database.

* **Local Writes:** All admin actions (adding students, sign-ins) write to local JSON first.
* **Cloud Mirror:** A background Go routine "de-bounces" changes and uploads a copy to a `LERN_Backup` folder on Google Drive.
* **Restoration:** Drive is only used if the local `./data` folder is lost or corrupted.
* **Memos Integration:** [Memos](https://usememos.com/docs/configuration/storage) will be configured for **Local Filesystem Storage**, allowing its images and SQLite DB to be backed up via the same Go routine.

---

## 3. MVP Feature Set
Beyond the existing [ClassGo](https://github.com/qiangli/classgo) attendance features, the MVP will include:

### **A. Identity & Academic Management**
* **Linked Profiles:** Student JSON files will include parent contact info and "Remaining Credits."
* **Academic Folder:** A JSON array for tracking grades and progress, viewable as a filtered feed in [Memos](https://usememos.com/docs/configuration/storage).

### **B. Scheduling & Room Logic**
* **Conflict Detection:** Logic to prevent double-booking a teacher or a room.
* **No-Show Alerts:** A dashboard view comparing the day's schedule vs. real-time sign-ins.

### **C. Communication & Billing**
* **Daily Digest:** An automated email to parents summarizing their child's sign-in/out times and any `#public` notes from Memos.
* **Credit System:** Automatic decrement of "Student Credits" based on total time recorded during sign-out.

---

## 4. Technical Roadmap

| Phase | Goal | Key Task |
| :--- | :--- | :--- |
| **Phase 1** | **Identity** | Expand ClassGo to support Student/Parent JSON CRUD. |
| **Phase 2** | **Integration** | Deploy [Memos](https://github.com/usememos/memos) as a sidecar; link `student_id` tags to local JSON. |
| **Phase 3** | **Cloud** | Implement the [Google Drive V3 API](https://developers.google.com/drive/api/v3/about-sdk) one-way sync. |
| **Phase 4** | **Automation** | Add the "Daily Digest" emailer and CSV import/export for monthly reporting. |

---

### **Final Design Note**
By keeping the **filesystem as the source of truth**, you ensure that even if the Go app or Memos database fails, the tutoring center’s data remains accessible as simple text files. This makes the app incredibly resilient for a small business environment.

How would you like to handle the **Google Drive** credentials—should the app use a shared **Service Account** (fixed for everyone) or a **User OAuth** flow (letting each center use their own account)?