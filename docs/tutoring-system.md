# TutorOS Architecture Specification v2.0

> **Status**: Design Complete | **Last Updated**: 2026-04-18

## Executive Summary

TutorOS is a lightweight, single-binary tutoring center management system built in Go. It provides scheduling, attendance tracking, billing, and basic reporting for small-to-medium tutoring operations (1-20 tutors, 50-1000 students).

**Core Philosophy**: *Simple enough for a tutoring center owner to run, powerful enough to manage growth.*

### Key Design Decisions

| Aspect | Decision | Rationale |
|--------|----------|-----------|
| **Deployment** | Single binary | Zero dependencies, copy-and-run simplicity |
| **Database** | SQLite (embedded) | No server setup, single-file backup, sufficient scale |
| **Auth** | Session cookies + bcrypt | Simple, debuggable, no external auth service |
| **Frontend** | Server-rendered + HTMX | Single binary, progressive enhancement, no build step |
| **Styling** | Tailwind CDN | Existing, works well, no build pipeline |
| **Emails** | SMTP configurable | Works with Gmail, SendGrid, or any provider |
| **Files** | Local filesystem | Simple, S3-compatible storage can be added later |

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          TUTOR OS (Single Binary)                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│   │  Web UI      │  │  API Layer   │  │  Background Workers      │  │
│   │  (Templates) │  │  (/api/v1/*) │  │  • Email sender          │  │
│   └──────┬───────┘  └──────┬───────┘  │  • Reminder scheduler    │  │
│          │                  │          │  • Invoice generator     │  │
│          └──────────────────┼──────────┴──────────────────────────┘  │
│                             │                                        │
│   ┌─────────────────────────▼─────────────────────────────────────┐  │
│   │                    Business Logic Layer                        │  │
│   │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐  │  │
│   │  │  Booking   │ │  Billing   │ │ Attendance │ │  Reporting │  │  │
│   │  │  Engine    │ │  System    │ │  Tracker   │ │   Engine   │  │  │
│   │  └────────────┘ └────────────┘ └────────────┘ └────────────┘  │  │
│   └─────────────────────────┬─────────────────────────────────────┘  │
│                             │                                        │
│   ┌─────────────────────────▼─────────────────────────────────────┐  │
│   │                      Data Layer                                │  │
│   │  ┌──────────────────┐    ┌─────────────────────────────────┐  │  │
│   │  │   SQLite (main)  │    │  File Storage                  │  │  │
│   │  │   • Users        │    │  • Export files (CSV, PDF)     │  │  │
│   │  │   • Sessions     │    │  • Photos/documents            │  │  │
│   │  │   • Attendance   │    └─────────────────────────────────┘  │  │
│   │  │   • Billing      │                                       │  │  │
│   │  │   • Audit Log    │    Optional Future:                  │  │  │
│   │  └──────────────────┘    ┌─────────────────────────────────┐  │  │
│   │                          │  S3-compatible object storage   │  │  │
│   │                          └─────────────────────────────────┘  │  │
│   └────────────────────────────────────────────────────────────────┘  │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Data Model

See the SQL schema in the implementation files for complete details. Key entities:

- **users** - Core accounts (admin, tutor, parent, student)
- **students** - Student profiles with emergency contacts
- **tutors** - Tutor profiles with rates and subjects
- **sessions** - Scheduled tutoring sessions
- **session_bookings** - Student registrations for sessions
- **attendance** - Check-in/out records
- **invoices** - Generated bills
- **payments** - Payment records

---

## API Design

### Authentication
- `POST /api/v1/auth/login` - Email/password login
- `POST /api/v1/auth/logout` - End session
- `GET /api/v1/auth/me` - Current user info

### Core Resources
- `/api/v1/users` - User management
- `/api/v1/students` - Student CRUD + history
- `/api/v1/tutors` - Tutor profiles + schedules
- `/api/v1/sessions` - Session scheduling + booking
- `/api/v1/attendance` - Check-in/out + reports
- `/api/v1/invoices` - Billing + payments

### Reports
- `/api/v1/reports/attendance` - CSV export
- `/api/v1/reports/revenue` - Revenue analysis
- `/api/v1/reports/tutor-hours` - Tutor utilization

---

## UI/UX Design

### Key Interfaces

1. **Mobile Sign-In** - QR code scan for quick student check-in
2. **Kiosk Mode** - Shared tablet interface with PIN pad
3. **Admin Dashboard** - Daily overview, quick actions, upcoming sessions
4. **Calendar View** - Weekly/monthly session scheduling
5. **Student Profile** - Complete history, attendance, billing
6. **Billing Center** - Invoices, payments, reports

---

## File Structure

```
classgo/
├── main.go                    # Entry point
├── internal/
│   ├── config/               # Configuration
│   ├── db/migrations/        # Schema migrations
│   ├── models/               # Data models
│   ├── auth/                 # Authentication
│   ├── services/             # Business logic
│   └── handlers/             # HTTP handlers
├── templates/                # HTML templates
├── static/                   # Static assets
└── docs/                     # Documentation
```

---

## Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)
- [ ] Migration system with versioned SQL files
- [ ] Structured logging (slog)
- [ ] Health check endpoint
- [ ] Configuration validation
- [ ] Docker setup

### Phase 2: User Management (Weeks 3-4)
- [ ] User, student, tutor schemas
- [ ] Password authentication (bcrypt)
- [ ] Session management
- [ ] Role-based access control
- [ ] Student/tutor CRUD UI

### Phase 3: Scheduling (Weeks 5-7)
- [ ] Session and booking tables
- [ ] Calendar view
- [ ] Student booking workflow
- [ ] Recurring sessions
- [ ] Conflict detection

### Phase 4: Attendance (Week 8)
- [ ] Enhanced kiosk with session selection
- [ ] Session-specific check-in/out
- [ ] Attendance reports
- [ ] Late notifications

### Phase 5: Billing (Weeks 9-10)
- [ ] Rate management
- [ ] Auto-invoice generation
- [ ] Payment recording
- [ ] PDF invoices
- [ ] A/R reporting

### Phase 6: Notifications (Weeks 11-12)
- [ ] SMTP configuration
- [ ] Email templates
- [ ] Session reminders
- [ ] Invoice emails
- [ ] In-app notifications

### Phase 7: Advanced (Future)
- [ ] Stripe integration
- [ ] Google Calendar sync
- [ ] Zoom integration
- [ ] Parent portal
- [ ] Progress tracking

---

## Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Database | SQLite | Embedded, zero-config, single-file backup |
| Auth | Session cookies | Simple, no external dependencies |
| Frontend | Server-rendered + HTMX | Keep single-binary, progressive enhancement |
| CSS | Tailwind CDN | Existing, no build step needed |
| Emails | SMTP | Works with any provider |
| Jobs | In-process goroutines | No Redis needed at this scale |

---

## Quick Start

```bash
# Build
go build -o classgo

# Run with defaults
./classgo

# Run with custom config
./classgo -config=config.prod.json

```

---
