# Crna Kobra

Ovaj repozitorijum sadrži postojeći statični sajt i razvojnu Go/PostgreSQL aplikaciju za sadržaj koji uređuje administrator.

## Grane

- `main` — postojeći javni statični sajt.
- `develop` — razvoj dinamičkog sadržaja. Ne prebacivati produkcioni radni direktorijum na ovu granu; za server koristiti poseban klon i poseban poddomen.

Commit na `develop` ne menja `main`. Stvarna baza sa sadržajem nije deo Git-a.

## Lokalno pokretanje dinamičkog sadržaja

Potrebni su Docker i Docker Compose. Iz korena repozitorijuma:

```bash
cp content-app/.env.example content-app/.env
```

U `content-app/.env` postaviti lokalne vrednosti:

```env
POSTGRES_PASSWORD=lokalna-dugacka-lozinka
ADMIN_EMAIL=vas-admin-email@example.com
ADMIN_PASSWORD=lokalna-admin-lozinka
```

Zatim pokrenuti:

```bash
docker compose --env-file content-app/.env -f docker-compose.content.yml up -d --build
```

Provera da li radi:

```bash
curl http://localhost:8090/health
```

Odgovor mora biti `ok`.

Korisne lokalne adrese:

- `http://localhost:8090/admin` — administratorski panel.
- `http://localhost:8090/vesti` — javni razvojni prikaz vesti.
- `http://localhost:8090/kampovi` — javni razvojni prikaz kampova.
- `http://localhost:8090/galerija` — javni razvojni prikaz galerije.

Za zaustavljanje:

```bash
docker compose --env-file content-app/.env -f docker-compose.content.yml down
```

Ova komanda zaustavlja kontejnere, ali ne briše bazu. Lokalni podaci su u `content-app/postgres-data/` i namerno nisu u Git-u.

## Administracija sadržaja

U `/admin` administrator može da kreira nacrt ili objavi:

- vest;
- kamp ili manifestaciju (naslov, datum, mesto, opis i naslovna fotografija);
- fotografiju kampa.

Fotografija kampa se vezuje za konkretan kamp kroz polje „Kamp kome pripada fotografija”. Objavljeni kampovi se prikazuju na `kampovi-i-manifestacije.html` u sekciji „Aktuelni kampovi i manifestacije”, a njihove fotografije u galeriji na istoj stranici.

Za lokalni pregled otvoriti `kampovi-i-manifestacije.html` u pregledaču dok Docker aplikacija radi, pa osvežiti stranicu nakon izmene u admin panelu.

## Važne napomene

- `content-app/.env`, `content-app/postgres-data/` i stvarni sadržaj baze se ne commitiju.
- Ovaj dinamički deo je razvojni prototip i još nije povezan sa produkcionim Nginx-om.
- Za server napraviti poseban klon, na primer `/home/krle/apps/crnakobra-develop`, pokrenuti njegov Docker Compose i povezati ga na poseban poddomen kao `develop.crnakobra.com`.
- Tek nakon provere se `develop` spaja u `main` i uvodi na glavni domen.

## Postojeći statični sajt

Statične stranice se nalaze u korenu repozitorijuma. Na postojećem serveru je uobičajeni tok: `git pull`, zatim kopiranje izmenjenih HTML/JS fajlova u `/home/krle/html/crnakobra/`. Ne kopirati razvojnu verziju na javni sajt dok se ne proveri na odvojenom okruženju.
