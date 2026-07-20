# r-a frontend

Frontend stack: React, TypeScript, Vite.  
Routing: TanStack Router. Data/cache: TanStack Query. PDF rendering: PDF.js.

## Run

```bash
npm install
npm run dev -- --port 5173
```

Open `http://localhost:5173`.

Auth pages: `/login`, `/register`.  
API URL: `VITE_API_URL` in `.env` (default `http://localhost:8080`).  
Backend must be running (`docker compose up` from repo root).

## Commands

```bash
npm run build    # Creates a production build in dist/
npm run preview  # Run this build locally
```
