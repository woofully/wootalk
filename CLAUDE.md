# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

### Backend (Go)
```bash
cd backend
go run main.go              # Run server on :8080
go build -o server main.go  # Build binary
go mod tidy                 # Update dependencies
```

### Frontend (Next.js)
```bash
cd frontend
npm install          # Install dependencies
npm run dev          # Dev server on :3000
npm run build        # Production build
npm run lint         # Run ESLint
```

### Environment Setup
Backend requires `DATABASE_URL` in `backend/.env` (copy from `.env.example`).

## Architecture

```
┌─────────────────┐     WebSocket      ┌─────────────────┐
│   Next.js       │◄──────────────────►│   Go Server     │
│   (port 3000)   │                    │   (port 8080)   │
└─────────────────┘                    └────────┬────────┘
                                                │ pgx/v5
                                                ▼
                                       ┌─────────────────┐
                                       │  PostgreSQL     │
                                       │  (Neon DB)      │
                                       └─────────────────┘
```

Anonymous 1-to-1 real-time chat with geolocation-based matching. Users identified by device UUID stored in localStorage (no accounts).

## Backend Structure (main.go)

All backend logic is in a single file with these key components:

- **Hub**: Central manager for users, matching queue, and database operations
- **User**: WebSocket connection with device ID, location, partner reference
- **Message routing**: Switch on message type in `handleWebSocket()`

Key flows:
1. **Matching**: `findMatch()` uses Haversine formula, prioritizes closer users within 1000km
2. **Session persistence**: 30-minute timeout, stores messages in PostgreSQL
3. **Reconnection**: `findActiveSession()` checks for resumable chats

## Frontend Structure (page.tsx)

Single-page app with all logic in one component:

- Device ID: Generated once, stored in `localStorage` as `wootalk_device_id`
- WebSocket: Connects on mount, handles all message types in `onmessage` switch
- State: `status` (disconnected/searching/connected), `messages[]`, `location`

## WebSocket Protocol

### Client → Server
| Type | Payload |
|------|---------|
| `connect` | `{device_id, latitude, longitude}` |
| `message` | `{content}` |
| `typing` / `stop_typing` | - |
| `disconnect` | - |

### Server → Client
| Type | Payload |
|------|---------|
| `matched` | `{content: distance_info}` |
| `message` | `{content, timestamp}` |
| `partner_left` / `searching` | - |
| `restore_session` | `{content, messages[]}` |
| `session_expired` / `error` | `{content}` |

## Database Schema

Three tables: `devices` (device_id, location, timestamps), `chat_sessions` (id, device_a, device_b, status), `messages` (chat_session_id, sender_device_id, content). Schema auto-creates on startup via `initDB()`.

## Testing

Open two browser tabs to simulate two users. Each generates a unique device ID and can be matched.
