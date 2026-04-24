# Feature Gap Analysis

This document identifies key features missing from ClassGo for a typical tutoring center operation. Features are organized by priority and category.

## Current Strengths

ClassGo covers operations well:

- Attendance check-in/check-out with PIN security and audit trails
- Task/tracker system with signoff enforcement
- Weekly schedule management with conflict detection
- Role-based access (admin, teacher, parent, student)
- Data import/export (XLSX/CSV)
- Automated backups with cloud sync
- Embedded Memos for general note-taking
- FRP tunnel for public access

## Priority Definitions

- **P0** -- Must-have for basic tutoring center operations
- **P1** -- Important for safety, trust, or daily workflow
- **P2** -- Valuable differentiator; improves retention and parent satisfaction
- **P3** -- Nice-to-have once the center scales

---

## P0: Billing & Payment Tracking

**Gap:** No financial tracking of any kind.

**What's needed:**

- Record tuition fees per student (monthly, per-session, or package-based)
- Track payment history and outstanding balances
- Generate simple invoices for parents
- Link attendance to billing (e.g., deduct from session packages on check-in)
- Payment status dashboard for admin

**Why it matters:** Every tutoring center needs to know who has paid and who hasn't. Without this, billing is tracked in a separate spreadsheet, defeating the single-system goal.

---

## P0: Parent Notifications

**Gap:** No outbound communication to parents.

**What's needed:**

- Email or SMS alert when a child checks in and checks out
- Configurable notification preferences per parent
- Notification delivery log (sent/failed)

**Why it matters:** Parents expect real-time visibility into their child's attendance. This is the single biggest trust-building feature for a tutoring center.

---

## P1: Absence Detection & Alerts

**Gap:** No mechanism to detect or flag expected-but-absent students.

**What's needed:**

- Compare today's schedule against actual check-ins
- Flag students who were expected but didn't arrive
- Notify admin and optionally parents of no-shows
- Track absence history per student

**Why it matters:** Safety concern. If a parent drops off a child and they don't check in, the center needs to know immediately.

---

## P1: Holiday & Closure Calendar

**Gap:** No way to mark days the center is closed or override the recurring schedule.

**What's needed:**

- Admin-managed calendar of holidays and closures
- Schedule system respects closures (don't show classes on closed days)
- Optional parent notification of upcoming closures
- Support for partial closures (e.g., morning only)

**Why it matters:** Without this, recurring schedules show classes on holidays, causing confusion for parents and teachers.

---

## P1: Per-Session Teacher Notes

**Gap:** Memos is a general note-taking app, not structured by session or student.

**What's needed:**

- Teachers record notes tied to a specific class session and date
- Notes visible to admin and optionally to parents
- Structured fields: topics covered, homework assigned, student behavior/progress
- Searchable history per student

**Why it matters:** "What did we work on today?" is a core tutoring question. Structured session notes are what differentiate a tutoring center from a babysitting service.

---

## P2: Progress Reports for Parents

**Gap:** No way to generate a summary of a student's progress over time.

**What's needed:**

- Aggregate attendance, task completion, and session notes into a per-student report
- Configurable time period (weekly, monthly, term)
- Exportable as PDF or printable HTML
- Optional teacher commentary section

**Why it matters:** Parents paying for tutoring want evidence of progress. A periodic report justifies tuition and builds retention.

---

## P2: Attendance Trend Reports

**Gap:** Basic daily metrics exist but no longitudinal analysis.

**What's needed:**

- Attendance frequency trends per student over weeks/months
- Average session duration trends
- Center-wide utilization (students per day, peak hours)
- Visual charts (or exportable data for external charting)
- Student retention/drop-off indicators

**Why it matters:** Helps admin spot students who are attending less frequently before they drop out entirely.

---

## P2: Academic Progress & Grading

**Gap:** The tracker system handles tasks but not academic assessment.

**What's needed:**

- Record test scores, quiz results, or skill assessments per student per subject
- Track progress over time with historical comparison
- Grade-level benchmarking (is the student at, above, or below grade level?)
- Include in progress reports

**Why it matters:** Tutoring centers focused on academics need to measure whether students are actually improving.

---

## P2: Student-Facing Learning Journal

**Gap:** Students have no view of their own learning history.

**What's needed:**

- Student-visible log of sessions attended, topics covered, homework assigned
- Task completion history (already partially exists via tracker)
- Personal goal tracking

**Why it matters:** Student ownership of learning improves engagement, especially for older students.

---

## P3: Enrollment & Waitlist Management

**Gap:** No capacity-based enrollment workflow.

**What's needed:**

- Enforce class capacity limits (rooms already have capacity field)
- Waitlist when a class is full, with automatic promotion when a spot opens
- Trial/demo session tracking for prospective students
- Enrollment status workflow: inquiry, trial, enrolled, withdrawn

**Why it matters:** Only needed once classes regularly hit capacity, but prevents over-enrollment and lost prospects.

---

## P3: Makeup Session Tracking

**Gap:** No mechanism to reschedule missed sessions.

**What's needed:**

- When a student is absent, offer to schedule a makeup session
- Track makeup credits (how many owed, how many used)
- Link makeups to the original missed session

**Why it matters:** Many centers offer makeup sessions as part of their policy. Without tracking, credits are lost or disputed.

---

## P3: Document Management

**Gap:** No file upload or attachment capability.

**What's needed:**

- Attach files to students (permission slips, assessment forms)
- Attach files to sessions (worksheets, homework handouts)
- Basic file storage with access control

**Why it matters:** Reduces paper handling, but most small centers can manage without it initially.

---

## P3: Multi-Language Support (i18n)

**Gap:** All UI text is English-only.

**What's needed:**

- Externalize all UI strings
- Support at least one additional language (e.g., Spanish, Chinese)
- Per-user language preference

**Why it matters:** Tutoring centers often serve multilingual families. Parent-facing features especially benefit from localization.

---

## P3: Announcement System

**Gap:** No way to broadcast messages to parents or students.

**What's needed:**

- Admin creates announcements (schedule changes, events, reminders)
- Display on dashboard and/or send via email/SMS
- Target by class, grade, or all students
- Archive of past announcements

**Why it matters:** Reduces individual phone calls and texts for common communications.

---

## Summary Table

| Priority | Feature                        | Category       |
|----------|--------------------------------|----------------|
| P0       | Billing & payment tracking     | Financial      |
| P0       | Parent notifications           | Communication  |
| P1       | Absence detection & alerts     | Safety         |
| P1       | Holiday & closure calendar     | Scheduling     |
| P1       | Per-session teacher notes      | Academic       |
| P2       | Progress reports for parents   | Reporting      |
| P2       | Attendance trend reports       | Reporting      |
| P2       | Academic progress & grading    | Academic       |
| P2       | Student learning journal       | Academic       |
| P3       | Enrollment & waitlist          | Enrollment     |
| P3       | Makeup session tracking        | Scheduling     |
| P3       | Document management            | Infrastructure |
| P3       | Multi-language support         | Infrastructure |
| P3       | Announcement system            | Communication  |
