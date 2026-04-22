# Unified Task Items: Design & Implementation

## Context

The tracker system was refactored from two tables (`tracker_items`, `student_tracker_items`) into one `task_items` table with a `scope` column. This document covers the full design including extensions for attribute-based targeting, task types, late signoff, and task groups.

## Scope Model

| Scope | Value | Who Creates | Who Sees | Key Field |
|-------|-------|-------------|----------|-----------|
| Center | 1 | Admin | All students (or filtered by criteria) | `criteria` |
| Class | 2 | Admin, Teacher | Enrolled students | `schedule_id` |
| Personal | 3 | Admin, Teacher, Parent, Student | One student | `student_id` |

## Task Types

Replaces the old `requires_signoff` boolean with explicit intent:

| Type | Checkout | Dashboard | UI Control | Blocks Checkout |
|------|----------|-----------|------------|-----------------|
| `todo` | Shown | Shown | Done / Undone (must pick one, undone with optional reason) | Yes |
| `task` | Not shown | Shown | Checkbox (optional completion) | No |
| `reminder` | Not shown | Shown | Info only (read) | No |

Stored as `type TEXT DEFAULT 'task'` on `task_items`. Hard-coded in Go for now. The checkout overlay is the student's task list filtered to `type='todo'`.

## Attribute-Based Criteria (Filtered Center Items)

A scope=1 item with `criteria` JSON is a center-wide item filtered by student attributes. No new scope values needed.

```json
{"grade_min": 9, "grade_max": 12}
{"birthplace": ["China", "Korea", "Japan"]}
{"grade_min": 10, "birthplace": ["China"]}
```

Supported keys: `grade_min`, `grade_max`, `birthplace[]`, `first_language[]`, `school[]`. New keys added by extending `matchesCriteria()` in Go — no schema change.

Matching is done in Go (not SQL) for simplicity: load student attributes once, iterate items.

## Late Signoff

When a student misses a checkout signoff, an admin/teacher can record it later:

- `tracker_responses.due_date` — the date it was originally due
- `tracker_responses.is_late` — 1 if signed off after due_date
- Recurrence checks use `COALESCE(due_date, response_date)` for backward compat

## Task Groups / Chains

Related items linked via `task_groups` table:

```sql
CREATE TABLE task_groups (
    id             TEXT PRIMARY KEY,   -- e.g., "college-essay-s001"
    name           TEXT NOT NULL,      -- "College Personal Statement"
    min_required   INTEGER,            -- NULL=all, 1=any one, N=at least N
    enforce_order  INTEGER DEFAULT 0   -- 1=sequential, 0=any order
);
```

Items reference via `group_id TEXT` + `group_order INTEGER` on `task_items`.

Dependencies are advisory — blocked items shown with visual indicator but still actionable.

## Schema

### task_items

| Column | Type | Default | Purpose |
|--------|------|---------|---------|
| id | INTEGER PK | AUTO | |
| scope | INTEGER | 1 | 1=center, 2=class, 3=personal |
| schedule_id | TEXT | NULL | for scope=2 |
| student_id | TEXT | NULL | for scope=3 |
| type | TEXT | 'task' | 'todo', 'task', 'reminder' |
| name | TEXT | NOT NULL | |
| notes | TEXT | NULL | |
| start_date | TEXT | NULL | |
| end_date | TEXT | NULL | |
| priority | TEXT | 'medium' | low, medium, high |
| recurrence | TEXT | 'daily' | daily, weekly, monthly, none |
| category | TEXT | NULL | |
| criteria | TEXT | NULL | JSON filter for scope=1 items |
| group_id | TEXT | NULL | links to task_groups.id |
| group_order | INTEGER | NULL | step number within group |
| created_by | TEXT | 'admin' | |
| owner_type | TEXT | 'admin' | |
| completed | INTEGER | 0 | for scope=3 one-time items |
| completed_at | TEXT | NULL | |
| completed_by | TEXT | NULL | |
| active | INTEGER | 1 | |
| deleted | INTEGER | 0 | |
| created_at | DATETIME | now | |
| updated_at | DATETIME | now | |

### tracker_responses

Existing columns plus:

| Column | Type | Default | Purpose |
|--------|------|---------|---------|
| due_date | TEXT | NULL | original due date (for late signoff) |
| is_late | INTEGER | 0 | 1 if signed off after due_date |

### task_groups

| Column | Type | Default | Purpose |
|--------|------|---------|---------|
| id | TEXT PK | | group identifier |
| name | TEXT | NOT NULL | display name |
| min_required | INTEGER | NULL | NULL=all, N=at least N |
| enforce_order | INTEGER | 0 | 1=sequential steps |

## Files

- `internal/models/models.go` — TaskItem, DueItem, TrackerResponse structs
- `internal/database/migrate.go` — schema, migrations
- `internal/database/tracker.go` — all CRUD and query logic
- `internal/database/seed.go` — sample data
- `internal/handlers/tracker.go` — API handlers
- `internal/handlers/dashboard.go` — dashboard API
- `templates/entry.html`, `templates/kiosk.html` — checkout overlay
- `templates/dashboard.html`, `templates/admin.html` — task management UI
