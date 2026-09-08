# Product Requirements Document (PRD)
**Product Name:** ATLAS (Attendance Tracking & Location-Authenticated System)
**Document Version:** 1.0
**Target Infrastructure:** 4GB RAM, 2 vCPU VPS (Docker Compose Deployment)

## 1. Product Overview
ATLAS (Attendance Tracking & Location-Authenticated System) is a modern, anti-proxy attendance tracking system designed for higher education institutions. It eliminates fraudulent check-ins (buddy punching) by combining Time-based One-Time Password (TOTP) dynamic QR codes with GPS geofencing. The system digitizes the entire attendance lifecycle, from daily check-ins to absence justifications and administrative analytics, distributed across a lightweight web and Progressive Web App (PWA) architecture.

## 2. User Roles & Permissions

| Role | Access Level | Primary Responsibilities |
| :--- | :--- | :--- |
| **System Admin** | System-Wide | Create classes, assign homeroom teachers, manage GPS perimeters, monitor college-wide analytics and audit logs. |
| **Teacher** | Homeroom Portal | Review/approve student absence requests, monitor class-level monthly/yearly analytics, flag chronic absenteeism. |
| **Class Rep** | Class Leader Portal | Initiate attendance sessions, generate dynamic anti-proxy QR codes, monitor real-time check-in feeds. |
| **Student** | Student PWA | Scan QR codes within classroom geofences, submit absence reasons with document proof, view personal attendance status. |

## 3. Core Functional Requirements

### 3.1. Anti-Proxy Attendance Engine
* **Dynamic TOTP QR:** The Class Rep's device generates a QR code payload that refreshes every 5 seconds. Static screenshots or video recordings are instantly invalidated.
* **GPS Geo-Fencing:** During the QR scan, the student's device must transmit its GPS coordinates. The server calculates the distance to the classroom center using the Haversine formula: 
  $$d = 2r rcsin\left(\sqrt{\sin^2\left(rac{\Delta lat}{2}ight) + \cos(lat_1)\cos(lat_2)\sin^2\left(rac{\Delta lng}{2}ight)}ight)$$
  Check-in is rejected if $d$ exceeds the administrative geofence radius (default: 30 meters).
* **Scan Performance:** Sub-2-second scan validation speed to handle morning rush concurrency without queueing.

### 3.2. Absence Request & Approval Pipeline
* **Submission:** Absent students must select a reason category (Sick, Outreach, Personal) and upload mandatory proof (medical certificates, official letters).
* **Teacher Review:** Teachers receive alerts for pending requests. Approving a request automatically converts the student's record from "Unexcused Absent" to "Excused Absent."
* **File Handling:** Uploaded documents are restricted to PDF/JPEG/PNG (max 5MB) and securely stored in an external S3-compatible cloud bucket (e.g., Cloudflare R2) to preserve VPS storage.

### 3.3. Analytics & Reporting
* **Teacher Insights:** Month-over-month trends, chronic absenteeism alerts (e.g., >15% unexcused), and absence category breakdowns.
* **Admin Dashboard:** College-wide compliance rates, active class counters, system health metrics, and chronological audit logs.

## 4. Notification Engine

| Trigger Event | Recipient | Channel | Payload / Action |
| :--- | :--- | :--- | :--- |
| **Absence Request Submitted** | Teacher | Push + In-App | "[Name] requested leave for [Date]. Tap to review." (Includes Approve/Reject inline actions) |
| **Absence Approved/Rejected** | Student | Push + In-App | "Your leave request for [Date] was [Approved/Rejected]." |
| **Unexcused Absence Recorded** | Student | Push + In-App | "You were marked absent for [Class]. Submit proof if applicable." |
| **Pending Requests Reminder** | Teacher | Email Digest | Daily 4:00 PM consolidation of pending approvals. |

## 5. User Interfaces & Flows

* **Class Rep (Mobile PWA):** Minimalist dashboard featuring a large, auto-refreshing QR code and a bottom drawer streaming a live feed of verified student check-ins.
* **Student (Mobile PWA):** Camera viewfinder interface indicating GPS lock status. Secondary screen for uploading absence justification forms.
* **Teacher (Web Portal):** Desktop-optimized dashboard highlighting daily attendance rates, a queue of pending absence requests with inline document viewers, and exportable analytics graphs.
* **Admin (Web Portal):** Global management interface to provision classes, define geofence coordinates, map teachers, and review system audit logs.

## 6. Technical Architecture & Tech Stack

Designed specifically for a resource-constrained VPS (4 GB ECC RAM, 2 Xeon vCPUs, 40 GB NVMe SSD).

* **Backend API:** Go (Golang) – Chosen for extreme memory efficiency (~50MB footprint) and high concurrency handling for morning check-in spikes.
* **Database:** PostgreSQL 16 – Tuned with a 1GB shared buffer for transactional integrity.
* **Cache:** Redis – Handles 5-second TOTP keys and API rate limiting (<60MB footprint).
* **Frontend:** SvelteKit or Vue 3 PWA – Offloads UI rendering entirely to the client device. Served as static files.
* **Reverse Proxy:** Nginx – Handles SSL termination and static file delivery (<30MB footprint).
* **Storage:** External S3/R2 Bucket – Offloads uploaded proof documents to prevent the 40GB NVMe drive from hitting capacity.

### 6.1. Server Memory Allocation Budget

| Service | Target RAM Limit | Primary Function |
| :--- | :--- | :--- |
| **PostgreSQL 16** | 1,024 MB | Core database storage, relationships, and indexing |
| **Go API Service** | 100 MB | Geofence math, API routing, TOTP validation |
| **Redis** | 128 MB | Temporary session keys and rate-limit tracking |
| **Nginx Proxy** | 32 MB | SSL termination and frontend delivery |
| **OS Buffer / Safety** | ~2,716 MB | System processes and OOM crash prevention |

## 7. Database Schema Core

* **`users`**: `id`, `full_name`, `email`, `role` (admin, teacher, class_rep, student), `class_id`.
* **`classes`**: `id`, `class_name`, `homeroom_teacher_id`, `geofence_lat`, `geofence_lng`, `geofence_radius_m`.
* **`qr_sessions`**: `id`, `class_id`, `totp_secret`, `session_start`, `session_end`.
* **`attendance_records`**: `id`, `student_id`, `class_id`, `status` (present, absent_unexcused, absent_excused), `scan_lat`, `scan_lng`, `recorded_at`.
* **`absence_requests`**: `id`, `student_id`, `attendance_id`, `reason_type`, `proof_file_url`, `status` (pending, approved, rejected).

## 8. API Endpoints Specification

All endpoints secured via HTTPS (TLS 1.3) and require JWT `Authorization: Bearer <token>`. Rate limiting is enforced at the Nginx/Redis layer (max 10 requests/min per IP for check-ins).

* **`POST /v1/qr/session`**: Generates new TOTP secret for the active class window. (Class Rep)
* **`POST /v1/attendance/scan`**: Validates student TOTP payload and device GPS against the class geofence. (Student)
* **`POST /v1/absences`**: Submits absence reason and S3 document URL. (Student)
* **`PATCH /v1/absences/{id}/review`**: Approves/rejects request and updates attendance state. (Teacher)
* **`POST /v1/admin/classes`**: Provisions class boundaries and assigns faculty. (Admin)

## 9. Deployment & Security Standards

* **Deployment Strategy:** Docker Compose. Strict avoidance of Kubernetes (K8s/K3s) to prevent control plane overhead from consuming the limited 4GB RAM. Explicit memory limits (`mem_limit`) enforced in `docker-compose.yml`.
* **Network Edge:** Cloudflare Enterprise routing for DDoS protection and Web Application Firewall (WAF) rule enforcement.
* **Encryption:** AES-256 encryption at rest for the PostgreSQL database and external document buckets.
* **Malware Protection:** Uploaded student documents are validated via strict MIME-type checking before transmission to cloud storage.
