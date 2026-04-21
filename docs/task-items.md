# Task Item Management

## Two Systems

### 1. Global Tracker Items (`tracker_items` table)

- Created by **admin only** via the Admin page > Progress section
- Apply to **all students** automatically -- no assignment needed
- Default recurrence: **daily**
- Managed at: `/api/v1/tracker/items` (admin-only endpoints)
- Show up as "Global" badge in student task lists

### 2. Personal Task Items (`student_tracker_items` table)

- Created by **any role** via Dashboard > My Items tab
- Two modes:
  - **Library item** -- `student_id` is empty, acts as a reusable template. Can later be assigned (copied) to students
  - **Assigned item** -- has a `student_id`, shows up in that student's task list
- Ownership tracked by `created_by` (entity ID) and `owner_type` (teacher/parent/student/admin)
- Only the creator (or admin) can edit/delete
- Managed at: `/api/tracker/student-items` and `/api/dashboard/teacher-items`

## How Each Role Uses Them

| Role | Can Create | Scope | Default Sign-off |
|------|-----------|-------|-----------------|
| **Admin** | Global items + personal items | All students | Yes |
| **Teacher** | Personal items | Their scheduled students | Yes |
| **Parent** | Personal items | Their children | Yes |
| **Student** | Personal items (self only) | Themselves only | No |

## Creation Flows

1. **Tasks tab > "Add Task"** -- create-and-assign in one step (student required)
2. **My Items tab > "Add Item"** -- student is optional; if blank, creates a library item
3. **My Items tab > "Assign"** -- copies a library item to selected students (teacher/parent only)
4. **Tasks tab > "Assign to Class"** -- bulk-assign to all students in a schedule (teacher only)
5. **Admin page > Tracker Items** -- CRUD global items that apply to everyone

## At Check-out Time

Both global and assigned personal items appear in the student's due list (filtered by recurrence logic -- daily/weekly/monthly/one-time). The student responds done/not-done to each. Items with `requires_signoff = false` (student-created private items) are excluded from the checkout sign-off flow.

## API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/tracker/items` | GET/POST | Admin: list/create global tracker items |
| `/api/v1/tracker/items/delete` | POST | Admin: soft-delete global item |
| `/api/tracker/student-items` | GET/POST | List/create personal task items |
| `/api/tracker/student-items/delete` | POST | Soft-delete personal item (owner only) |
| `/api/tracker/complete` | POST | Mark one-time task done/undone |
| `/api/dashboard/teacher-items` | GET | List all items created by logged-in user |
| `/api/dashboard/assign-library-item` | POST | Copy library item to selected students |
| `/api/dashboard/bulk-assign` | POST | Assign task to all students in a class |
| `/api/dashboard/all-tasks` | GET | All tasks (global + personal) for a student |
| `/api/tracker/due` | GET | Due items for today (recurrence-aware) |

## Database Tables

### `tracker_items` (Global)

Key columns: `name`, `notes`, `start_date`, `end_date`, `priority`, `recurrence` (default: daily), `category`, `created_by` (default: admin), `active`, `deleted`

### `student_tracker_items` (Personal)

Key columns: `student_id` (empty for library items), `name`, `notes`, `start_date`, `end_date`, `priority`, `recurrence` (default: none), `category`, `created_by`, `owner_type`, `requires_signoff` (default: true), `completed`, `completed_at`, `completed_by`, `active`, `deleted`

### `tracker_responses` (Completion Tracking)

Key columns: `student_id`, `student_name`, `item_type` (global/personal), `item_id`, `item_name`, `status` (done/not_done), `notes`, `response_date`, `attendance_id`
