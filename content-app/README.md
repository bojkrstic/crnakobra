# Dinamički sadržaj — razvojna osnova

Ovo je nova, odvojena Go aplikacija za sadržaj koji uređuje administrator. Ne zamenjuje niti trenutno menja statični sajt u korenu repozitorijuma.

Prva faza uvodi PostgreSQL model za:

- `news` — vesti;
- `camp` — kampove i manifestacije;
- `gallery` — stavke galerije.

Administrator se prijavljuje na `http://localhost:8090/admin`. Panel omogućava unos, izmenu, objavu i brisanje vesti, kampova i stavki galerije, uključujući upload JPEG, PNG ili WebP slike. Slike se u ovoj razvojnoj fazi čuvaju u PostgreSQL bazi.

## Lokalno pokretanje

```bash
cp content-app/.env.example content-app/.env
# U content-app/.env upisati lokalne POSTGRES_PASSWORD, ADMIN_EMAIL i ADMIN_PASSWORD vrednosti.
docker compose --env-file content-app/.env -f docker-compose.content.yml up --build
```

Nakon pokretanja: `http://localhost:8090/health` mora vratiti `ok`. Javne razvojne rute su `/vesti`, `/kampovi` i `/galerija`.

Za gašenje koristiti `docker compose -f docker-compose.content.yml down`. Ne brisati `content-app/postgres-data/` ako želite sačuvati lokalne unose.

## Planirana izolacija

U produkciji aplikacija ostaje na internom portu, a Nginx će joj prosleđivati samo eksplicitno definisane rute (`/admin`, `/vesti`, `/kampovi`, `/galerija`). Postojeći statični fajlovi nastavljaju da se poslužuju kao sada.
