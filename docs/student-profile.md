# Student Profile System

## Summary

The student profile system implements the full student intake form. It covers schema expansion, a user-facing profile form with view/edit modes, draft/finalize review workflow, tracker item integration for academic data, and auto-assignment of tasks based on profile gaps.

## Architecture

### Data Storage: Two-Layer Approach

**Layer 1 — Student/Parent table columns** (static personal info):

| Field | Table | Purpose |
|---|---|---|
| dob | students | Date of birth |
| birthplace | students | Where born |
| years_in_us | students | Years lived in US |
| first_language | students | First language learned |
| previous_schools | students | Semicolon-separated list |
| courses_outside | students | Courses outside high school |
| profile_status | students | "", "draft", or "final" |
| email2, phone2 | parents | Second parent/guardian contact |

These extend the existing student fields (id, first_name, last_name, grade, school, parent_id, email, phone, address, notes, active).

**Layer 2 — Tracker items** (academic data that changes over time):

21 predefined global tracker items seeded by `SeedSampleData()`:

| Category | Items | Recurrence |
|---|---|---|
| GPA | Weighted GPA, Unweighted GPA | monthly |
| PSAT | 8/9, 10, 11/NMSQT | none (one-time) |
| SAT | 1st Attempt, 2nd Attempt | none |
| AP | Exam Score, In-Progress | none |
| Math Competition | AMC/AIME | none |
| Extracurricular | Talents, Clubs, Sports, Leadership, Volunteer | monthly |
| College Prep | Awards, Internship, Summer, Major Interest | mixed |
| Personal | Hobbies, Favorite Subjects | none |

Values entered on the profile form are saved as `tracker_responses` rows (item_type='global', value in notes field).

---

## Signup / Login

### Signup (new users)

**Location:** Root check-in page (`/`) — "Sign Up" link toggles the signup form.

**Flow:**
1. User enters **Last Name**, **First Name**, **Password** (4+ chars)
2. POST `/api/login` with `action: "signup"`
3. Backend searches students/parents/teachers by name (case-insensitive, `deleted=0`)
4. If match found and no existing account → creates Memos user, sets password, creates session
5. If match found but account exists → error: "Account already exists. Please use Log In instead."
6. If no match → error: "No student found with that name."
7. On success → redirect to `/profile` (edit mode, since profile is empty)

**API:**
```
POST /api/login
{
  "action": "signup",
  "first_name": "Alice",
  "last_name": "Wang",
  "password": "mypass"
}
→ {"ok": true, "role": "user", "redirect": "/profile"}
```

### Login (returning users)

**Location:** `/login` page — search by name/ID, enter password.

**Flow:**
1. User searches for themselves via typeahead (searches students/parents/teachers)
2. Selects their entry, enters password
3. POST `/api/login` with `action: "login"`, `entity_id`, `password`
4. Backend validates password via Memos user store (bcrypt)
5. On success → redirect to `/dashboard` (tasks page)

**API:**
```
POST /api/login
{
  "action": "login",
  "entity_id": "S001",
  "password": "mypass"
}
→ {"ok": true, "role": "user", "redirect": "/dashboard"}
```

### Admin Login

**Location:** `/login` page — "Admin Login" toggle.

**Flow:** System username + password → OS-level authentication (macOS dscl / Linux PAM) → redirect to `/admin`.

### Session

- Cookie: `classgo_session` (HttpOnly, 8hr expiry)
- In-memory session store with Role (admin/user), UserType (student/parent/teacher), EntityID

---

## Profile Pages

### User Profile (`/profile`) — RequireAuth

For students and parents. Accessible from dashboard sidebar "Profile" link.

**View mode** (default for returning users with filled profile):
- All fields displayed as read-only text
- "Edit" button to switch to edit mode

**Edit mode** (default for first-time / empty profile):
- All fields as form inputs
- "Save" and "Cancel" buttons
- Save sets `profile_status = 'draft'`

**Sections** (matching the .docx intake form):
1. Personal Information — name, DOB, birthplace, years in US, first language, email, phone, address
2. Parent/Guardian — names, email/phone (primary + secondary), address
3. High School Education — school, grade, previous schools, courses outside
4. GPA — weighted/unweighted (tracker items)
5. Standardized Tests — PSAT, SAT (tracker items)
6. AP Exams — completed + in-progress (tracker items)
7. Math Competitions — AMC/AIME (tracker items)
8. Extracurricular Activities — talents, clubs, sports, leadership, volunteer (tracker items)
9. College Prep — awards, internship, summer, majors (tracker items)
10. Personal — hobbies, favorite subjects (tracker items)
11. Notes

**Access control:**
- Students see/edit only their own profile
- Parents see/edit their children's profiles (child selector dropdown if multiple)

**API:**
```
GET  /api/v1/user/profile?student_id=S001
POST /api/v1/user/profile
  { student: {...}, parent: {...}, tracker_values: {"1": "3.8", "5": "E:720 M:750 T:1470"} }
```

### Admin Profile (`/admin/profile?id=X`) — RequireAdmin

Same form as user profile plus:
- **Finalize button** — marks profile as reviewed/approved (`profile_status = 'final'`)
- **Status badge** — Draft (amber), Final (green)
- Accessible from admin Data tab → student row → "Profile" link

**API:**
```
POST /api/v1/student/profile
  { student: {...}, parent: {...}, finalize: true }
```

---

## Draft/Finalize Workflow

| Status | Meaning | Who sets it |
|---|---|---|
| `""` (empty) | No profile submitted | Default |
| `"draft"` | Student/parent submitted, pending review | Set on user profile save |
| `"final"` | Admin/teacher reviewed and approved | Set by admin via Finalize |

- Admin Data tab shows "Profile" column (Draft/Final/empty)
- Students see "Draft — pending review" banner until finalized
- Students can still edit while in draft (corrections before finalization)

---

## Auto-Plan: Task Generation from Profile Gaps

On profile save, `AutoAssignProfileTasks()` inspects what's missing and creates `student_tracker_items`:

**Logic:**
1. For each of the 21 global tracker items, check if student has a response or existing assignment
2. If missing → auto-create a one-time student_tracker_item (created_by='system')
3. Grade-aware filtering:
   - PSAT 8/9: only if grade <= 9
   - PSAT 10: only if grade >= 10
   - PSAT 11: only if grade >= 11
   - SAT: only if grade >= 10
   - AP: only if grade >= 9

Auto-assigned items appear on the student's dashboard Tasks section, prompting them to complete their profile.

---

## Key Files

| File | Purpose |
|---|---|
| `internal/models/models.go` | Student/Parent structs with new fields |
| `internal/database/migrate.go` | Schema + migrations for new columns |
| `internal/database/seed.go` | 21 predefined global tracker items |
| `internal/database/tracker.go` | GetGlobalTrackerItems, GetLatestTrackerValues, SaveProfileTrackerValues, AutoAssignProfileTasks |
| `internal/handlers/profile.go` | Admin + user profile handlers, access control, auto-plan |
| `internal/handlers/app.go` | Signup action in HandleLoginAPI, findEntityByName |
| `internal/datastore/reader.go` | Import new fields from CSV/XLSX |
| `internal/datastore/writer.go` | Export new fields + queryStudents/queryParents |
| `internal/datastore/importer.go` | Row hash + upsert with new fields |
| `templates/mobile.html` | Signup form on check-in page |
| `templates/user_profile.html` | User-facing profile form (view/edit modes) |
| `templates/profile.html` | Admin profile page (with Finalize) |
| `templates/dashboard.html` | Profile nav item in sidebar |
| `templates/admin.html` | Profile status column + Profile link in data tab |
| `main.go` | Route registration for /profile, /api/v1/user/profile |
| `data/csv.example/students.csv` | Example data with new columns |
| `data/csv.example/parents.csv` | Example data with email2/phone2 |

---

## Routes Summary

| Route | Auth | Purpose |
|---|---|---|
| `POST /api/login` (signup) | Public | Create account by name |
| `POST /api/login` (login) | Public | Authenticate returning user |
| `GET /profile` | RequireAuth | User profile page |
| `GET/POST /api/v1/user/profile` | RequireAuth | User profile API (own/children only) |
| `GET /admin/profile?id=X` | RequireAdmin | Admin profile page |
| `GET/POST /api/v1/student/profile` | RequireAdminAPI | Admin profile API (any student) |
