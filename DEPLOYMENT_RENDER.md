# Deploy WooTalk on Render (Free Tier)

This guide will help you deploy your chat application on Render's free tier.

## Overview

You'll deploy:
1. **PostgreSQL Database** (Free tier: 90 days, then $7/month)
2. **Backend Go Server** (Free tier: spins down after inactivity)
3. **Frontend Next.js** (Free tier: static site or web service)

---

## Step 1: Prepare Your Code

### 1.1 Commit Changes
The code has been updated for Render compatibility. Commit and push:

```bash
git add .
git commit -m "Configure for Render deployment"
git push origin main
```

---

## Step 2: Create PostgreSQL Database

### 2.1 Go to Render Dashboard
1. Go to https://render.com and sign in
2. Click **"New +"** → **"PostgreSQL"**

### 2.2 Configure Database
- **Name**: `wootalk-db`
- **Database**: `wootalk`
- **User**: `wootalk` (auto-generated)
- **Region**: Choose closest to your users (e.g., Singapore, Oregon)
- **Plan**: **Free** (90-day trial)

### 2.3 Create Database
1. Click **"Create Database"**
2. Wait for it to provision (takes 1-2 minutes)
3. **Save the Internal Database URL** - you'll need this for the backend

**Location**: Dashboard → Database → "Info" tab → "Internal Database URL"

Example format:
```
postgresql://wootalk_user:password@dpg-xxxxx-a.singapore-postgres.render.com/wootalk_db
```

---

## Step 3: Deploy Backend

### 3.1 Create Web Service
1. Click **"New +"** → **"Web Service"**
2. Connect your GitHub repository (woofully/wootalk)
3. Click **"Connect"** next to your repository

### 3.2 Configure Backend Service
Fill in these settings:

**Basic Settings:**
- **Name**: `wootalk-backend`
- **Region**: Same as database (e.g., Singapore)
- **Branch**: `main`
- **Root Directory**: Leave blank (the Dockerfile is in root now)
- **Environment**: **Docker**
- **Plan**: **Free**

**Advanced Settings:**
- **Health Check Path**: `/health`
- **Auto-Deploy**: Yes (optional - deploys on git push)

### 3.3 Add Environment Variables
Click **"Advanced"** → **"Add Environment Variable"**:

| Key | Value |
|-----|-------|
| `DATABASE_URL` | Paste the Internal Database URL from Step 2.3 |

### 3.4 Deploy
1. Click **"Create Web Service"**
2. Wait for build and deployment (5-10 minutes first time)
3. Monitor logs for any errors
4. Once deployed, **copy the service URL** (e.g., `https://wootalk-backend.onrender.com`)

---

## Step 4: Deploy Frontend

You have two options:

### Option A: Deploy as Static Site (Recommended for Free Tier)

#### 4.1 Update Frontend Environment
Update `frontend/.env.local` with your backend URL:
```bash
NEXT_PUBLIC_WS_URL=wss://wootalk-backend.onrender.com/ws
```

Commit and push:
```bash
git add frontend/.env.local
git commit -m "Update WebSocket URL for Render backend"
git push origin main
```

#### 4.2 Deploy on Vercel (Free)
Vercel is perfect for Next.js and has better free tier than Render for frontend:

1. Go to https://vercel.com
2. Click **"Add New Project"**
3. Import your GitHub repository
4. **Framework Preset**: Next.js (auto-detected)
5. **Root Directory**: `frontend`
6. **Environment Variables**: Add `NEXT_PUBLIC_WS_URL=wss://wootalk-backend.onrender.com/ws`
7. Click **"Deploy"**

### Option B: Deploy Frontend on Render

#### 4.1 Create Web Service
1. In Render dashboard, click **"New +"** → **"Web Service"**
2. Connect same repository
3. Name it `wootalk-frontend`

#### 4.2 Configure
- **Root Directory**: `frontend`
- **Environment**: **Node**
- **Build Command**: `npm install && npm run build`
- **Start Command**: `npm start`
- **Plan**: **Free**

#### 4.3 Add Environment Variable
- **Key**: `NEXT_PUBLIC_WS_URL`
- **Value**: `wss://wootalk-backend.onrender.com/ws`

#### 4.4 Deploy
Click **"Create Web Service"**

---

## Step 5: Test Your Deployment

### 5.1 Check Backend
Visit: `https://wootalk-backend.onrender.com/health`

Should return:
```json
{"status":"ok"}
```

### 5.2 Check Frontend
1. Open your frontend URL
2. Allow location access
3. Open a second browser/tab in incognito mode
4. Both should connect and get matched
5. Try sending messages

---

## Important Notes About Render Free Tier

### Backend Limitations
- **Spins down after 15 minutes of inactivity**
- **50-second delay on first request** after spin-down
- Monthly: 750 hours free (enough for continuous use)

### Solutions for Spin-down
If you want to prevent spin-down:

1. **Upgrade to Starter plan** ($7/month - no spin-down)
2. **Use a cron job** to ping your backend every 14 minutes:
   ```bash
   # Use a service like cron-job.org to hit:
   # https://wootalk-backend.onrender.com/health
   ```

### Database Limitations
- **Free for 90 days**, then $7/month
- 1GB storage
- Shared CPU
- Perfect for development/testing

---

## Troubleshooting

### Backend Deployment Failed
- Check logs in Render dashboard
- Verify `DATABASE_URL` is set correctly
- Make sure all code is committed and pushed

### Frontend Can't Connect to Backend
- Verify `NEXT_PUBLIC_WS_URL` uses `wss://` (not `ws://`)
- Check backend is running and healthy
- Check browser console for WebSocket errors

### Database Connection Error
- Verify you're using the **Internal Database URL** (not external)
- Check database is running in Render dashboard
- Make sure URL includes `?sslmode=require` if needed

### CORS Errors
The backend already allows all origins. If you see CORS errors:
- Make sure you're using WebSocket (`wss://`), not HTTP
- Check browser console for actual error message

---

## Monitoring

### View Logs
1. Go to Render dashboard
2. Click on your service
3. Click **"Logs"** tab
4. Monitor real-time logs

### Check Database
1. Go to database in Render dashboard
2. Click **"Connect"** to use psql
3. Or use the external connection string with any PostgreSQL client

---

## Cost Summary

| Service | Free Tier | Paid Option |
|---------|-----------|-------------|
| Backend | 750 hrs/month (spins down) | $7/month (no spin-down) |
| Database | 90 days free | $7/month after trial |
| Frontend (Vercel) | Unlimited | Starts at $20/month for pro features |
| Frontend (Render) | 750 hrs/month | $7/month |

**Total for truly free setup**: Frontend on Vercel (always free) + Backend on Render free tier

---

## Next Steps

1. Consider adding a domain name (Render allows custom domains on free tier)
2. Set up monitoring/alerting
3. Add analytics
4. Implement rate limiting
5. Add more robust error handling

---

## Support

If you run into issues:
- Check Render documentation: https://render.com/docs
- Check Render status: https://status.render.com
- Render community: https://community.render.com
