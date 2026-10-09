# JST - Job Search Tool
A local job search tool for aggregating postings, application preparation, and tracking status/outcomes.

## Run with Docker
Requires Docker Desktop w/ Docker Compose.

From the repo root:
```sh
mkdir -p data
docker compose up --build --wait
```

Check http://127.0.0.1:8080 - confirm the health page shows backend up.

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

Open http://127.0.0.1:5173.

Run either the native backend or the Compose application at a time.