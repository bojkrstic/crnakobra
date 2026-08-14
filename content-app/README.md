# Dinamički sadržaj — razvojna osnova

Ovo je nova, odvojena Go aplikacija za sadržaj koji uređuje administrator. Ne zamenjuje niti trenutno menja statični sajt u korenu repozitorijuma.

Prva faza uvodi PostgreSQL model za:

- `news` — vesti;
- `camp` — kampove i manifestacije;
- `gallery` — stavke galerije.

Sledeća celina je prijava administratora i CRUD panel. Tek nakon toga se javne rute pažljivo povezuju sa postojećim stranicama.

## Lokalno pokretanje

```bash
cp content-app/.env.example content-app/.env
# U content-app/.env upisati stvarnu, lokalnu POSTGRES_PASSWORD vrednost.
docker compose --env-file content-app/.env -f docker-compose.content.yml up --build
```

Nakon pokretanja: `http://localhost:8090/health` mora vratiti `ok`.

Za gašenje koristiti `docker compose -f docker-compose.content.yml down`. Ne brisati `content-app/postgres-data/` ako želite sačuvati lokalne unose.

## Planirana izolacija

U produkciji aplikacija ostaje na internom portu, a Nginx će joj prosleđivati samo eksplicitno definisane rute (`/admin`, `/vesti`, `/kampovi`, `/galerija`). Postojeći statični fajlovi nastavljaju da se poslužuju kao sada.
