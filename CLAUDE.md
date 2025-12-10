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
Backend requires `DATABASE_URL` in `backend/.env`:
```
DATABASE_URL=postgresql://user:pass@host/db?sslmode=require
```

Frontend requires `NEXT_PUBLIC_WS_URL` in `frontend/.env.local`:
```
NEXT_PUBLIC_WS_URL=ws://localhost:8080/ws              # Local development
NEXT_PUBLIC_WS_URL=wss://wootalk-backend.fly.dev/ws   # Production
```

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

Three tables auto-created on startup via `initDB()`:

```sql
devices (device_id PRIMARY KEY, latitude, longitude, created_at, last_seen)
chat_sessions (id PRIMARY KEY, device_a, device_b, status, created_at, updated_at)
messages (id SERIAL, chat_session_id, sender_device_id, content, created_at)
```

## Key Implementation Details

**Matching algorithm** (`findMatch()` in main.go):
- First pass: Find closest user within 1000km radius
- Fallback: Accept first available user if no nearby match
- Distance display: "<1 km", "nearby" (<10km), "in your region" (<100km), "in your country"

**Session persistence**:
- 30-minute sliding window based on `updated_at` timestamp
- Auto-restore if partner reconnects within window
- Session expires if partner is offline beyond timeout

**Typing indicators**:
- Frontend debounces with 2-second timeout before auto `stop_typing`
- Prevents excessive WebSocket messages

## Deployment

Backend is deployed on Render using Docker (configured in `render.yaml` and root `Dockerfile`):
- PostgreSQL database on Render (free tier: 90 days)
- Backend Go service with WebSocket support
- Frontend on Vercel or Render

See `DEPLOYMENT_RENDER.md` for complete step-by-step deployment instructions.

Quick reference:
```bash
# Backend uses PORT environment variable (auto-set by Render)
# Database URL must be set as environment variable in Render dashboard
# Frontend needs NEXT_PUBLIC_WS_URL=wss://your-backend.onrender.com/ws
```

## Testing

Open two browser tabs to simulate two users. Each generates a unique device ID and can be matched.
