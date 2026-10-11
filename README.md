# JST - Job Search Tool
A local job search tool for aggregating postings, application preparation, and tracking status/outcomes.

## Run with Docker
Requires Docker Desktop w/ Docker Compose.

From the repo root:
```sh
mkdir -p data
docker compose up --build --wait
```

Open http://127.0.0.1:8080 — it redirects to the Postings page.
Open http://127.0.0.1:8080/status to check the backend connection.

```sh
docker compose down
```

## Dev
Requires Go 1.27.1 and Node 22.22.3

Backend, from the repo root:
```sh
go run ./cmd/server
```

Frontend, in another terminal & from repo root
```sh
nvm use
cd web
npm ci
npm run dev
```

Open http://127.0.0.1:5173 — it redirects to the Postings page.
Open http://127.0.0.1:5173/status to check the backend connection.
Vite handles unknown page URLs through the frontend during development. Verify server 404 responses using the Compose application.

Run either the native backend or the Compose application at a time.