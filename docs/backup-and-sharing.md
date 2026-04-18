To fulfill your requirements for a local-first, **[ClassGo](https://github.com/qiangli/classgo)**-based system with a "hassle-free" cloud layer, here is the summarized architecture and execution plan.

---

## 1. Core Architecture
The app functions as a **Local-First hybrid**. Your Go backend manages the business logic and the filesystem, while Google Drive acts as a **passive mirror** for data safety and external "read-only" views.

* **Primary Database:** [modernc.org/sqlite](https://modernc.org/sqlite) (Pure Go) used for high-speed indexing.
* **Source of Truth:** A local `./data` folder containing human-readable JSON files.
* **User Interface:** [Memos](https://usememos.com/docs/configuration/storage) (configured for Local Filesystem storage) handles the teacher/student timeline and progress notes.

---

## 2. Google Drive Backup & Sharing Logic
To maintain a "hassle-free" experience, the app uses [OAuth 2.0](https://developers.google.com/identity/protocols/oauth2) to connect to the Admin's account.

### **The "One-Way Mirror" Strategy**
* **Automated Sync:** The Go backend watches for local file changes. When a student signs out or a teacher saves a note, the file is pushed to Google Drive in the background.
* **Disaster Recovery:** Drive is only used to **Restore** data if the local laptop's filesystem is corrupted or lost.

### **Automated View Sharing**
The app uses the [Google Drive Permissions API](https://developers.google.com/drive/api/v3/reference/permissions) to automate different "Views" without manual admin intervention:

| Audience | Access Method | What They See |
| :--- | :--- | :--- |
| **Admin** | **Root Folder Access** | All raw data, billing, and system configs. |
| **Tutors** | **Shared Sub-folder** | Daily attendance logs and schedules. |
| **Parents** | **Individual Folder** | Specific progress reports (PDFs) and their child's attendance history. |

---

## 3. Security & Safety Guardrails
To prevent "accidental sharing" with the wrong person, the app implements programmatic controls:

* **Programmatic Provisioning:** Only the Go app can create or share folders. This removes human error from the Google Drive UI.
* **Permission Scoping:** The app uses the `drive.appdata` scope (hidden storage) or specific `reader` roles for parents to ensure they cannot delete or modify records.
* **Visibility Toggles:** Only memos tagged `#public` or `#parent` in the [Memos UI](https://usememos.com/docs/configuration/storage) are exported to the shared Google Drive folders.

---

## 4. MVP Feature Summary
1.  **Kiosk Mode:** Tablet-optimized student sign-in/out (based on **[ClassGo](https://github.com/qiangli/classgo)**).
2.  **Profile Management:** Admin-only CRUD for staff, teachers, and student/parent links.
3.  **Conflict-Free Scheduling:** Room and tutor availability tracking on the filesystem.
4.  **Credit Tracking:** Automatic deduction of prepaid hours upon student sign-out.
5.  **Cloud-Mirror:** Zero-config background backup to the Admin's Google Drive.

---

## 5. Implementation Roadmap
* **Step 1:** Expand the `classgo` JSON schema to include `parent_email` and `credits`.
* **Step 2:** Integrate the [Google Drive API](https://developers.google.com/workspace/drive/api/guides/manage-sharing) permissions module.
* **Step 3:** Set up a [Memos](https://usememos.com/docs/configuration/storage) instance with local storage pointing to your Go data directory.
* **Step 4:** Build the "Export to PDF/CSV" routine that places clean reports into the shared Drive folders.

This plan delivers a professional, cloud-synced experience for users while keeping the technical complexity hidden and the data 100% under the tutoring center's control.

