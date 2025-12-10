# Quick Start - Deploy on Render

## TL;DR - 5 Minute Setup

### 1. Push Your Code
```bash
git push origin main
```

### 2. Create Database (2 min)
1. Go to https://render.com → New + → PostgreSQL
2. Name: `wootalk-db`, Plan: **Free**
3. Click **Create Database**
4. **Copy the Internal Database URL** from Info tab

### 3. Deploy Backend (2 min)
1. Render dashboard → New + → Web Service
2. Connect your GitHub repo: `woofully/wootalk`
3. Settings:
   - Name: `wootalk-backend`
   - Environment: **Docker**
   - Plan: **Free**
   - Health Check Path: `/health`
4. Environment Variables:
   - `DATABASE_URL` = (paste your database URL)
5. Click **Create Web Service**
6. Wait for deployment (watch logs)
7. **Copy your backend URL**: `https://wootalk-backend.onrender.com`

### 4. Update Frontend Config
```bash
# Edit frontend/.env.local
echo 'NEXT_PUBLIC_WS_URL=wss://wootalk-backend.onrender.com/ws' > frontend/.env.local

git add frontend/.env.local
git commit -m "Update backend URL"
git push
```

### 5. Deploy Frontend on Vercel (1 min)
1. Go to https://vercel.com → New Project
2. Import your GitHub repo
3. Settings:
   - Root Directory: `frontend`
   - Framework: Next.js (auto-detected)
4. Environment Variables:
   - `NEXT_PUBLIC_WS_URL` = `wss://wootalk-backend.onrender.com/ws`
5. Click **Deploy**

### Done! 🎉

Visit your Vercel URL and test the chat!

---

## Important: First Load Will Be Slow

Render free tier **spins down after 15 minutes** of inactivity.

First request after spin-down = **50 second delay** ⏰

### Solutions:
1. **Accept it** - it's free! Just wait 50 seconds on first load
2. **Use cron-job.org** to ping `/health` every 14 minutes
3. **Upgrade to $7/month** - no spin-down

---

## Test Your Deployment

1. Backend health check: `https://wootalk-backend.onrender.com/health`
   - Should return: `{"status":"ok"}`

2. Open frontend in 2 browser tabs (one in incognito)
3. Allow location access
4. Both users should match automatically
5. Send messages!

---

## Need Help?

See full guide: `DEPLOYMENT_RENDER.md`

## Costs

- **Backend**: Free (750 hrs/month, spins down)
- **Database**: Free for 90 days, then $7/month
- **Frontend (Vercel)**: Free forever
- **Total now**: $0
- **Total after 90 days**: $7/month (just database)
