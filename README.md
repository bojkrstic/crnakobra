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

Fotografija kampa se vezuje za konkretan kamp kroz polje „Kamp kome pripada fotografija”. Objavljeni sadržaj se može pregledati na razvojnim rutama aplikacije (`/kampovi` i `/galerija`).

Na `main` grani postojeći statični HTML nije povezan sa ovom aplikacijom. Povezivanje dinamičkih blokova sa stranicom kampova ostaje zaseban razvojni korak na `develop` grani.

## Važne napomene

- `content-app/.env`, `content-app/postgres-data/` i stvarni sadržaj baze se ne commitiju.
- Ovaj dinamički deo je razvojni prototip i još nije povezan sa produkcionim Nginx-om.
- Za server napraviti poseban klon, na primer `/home/krle/apps/crnakobra-develop`, pokrenuti njegov Docker Compose i povezati ga na poseban poddomen kao `develop.crnakobra.com`.
- Tek nakon provere se `develop` spaja u `main` i uvodi na glavni domen.

## Postojeći statični sajt

Statične stranice se nalaze u korenu repozitorijuma. Na postojećem serveru je uobičajeni tok: `git pull`, zatim kopiranje izmenjenih HTML/JS fajlova u `/home/krle/html/crnakobra/`. Ne kopirati razvojnu verziju na javni sajt dok se ne proveri na odvojenom okruženju.

## Kako ažurirati javni sajt

Ove naredbe se pokreću nakon prijavljivanja na server. Javna verzija sajta se služi iz `/home/krle/html/crnakobra/`.

```bash
# 1. Uđi u klon sajta na serveru.
cd ~/apps/crnakobra/crnakobra

# 2. Preuzmi poslednje izmene sa GitHub-a.
git pull

# 3. Kopiraj statične stranice, skripte i sitemap u javni HTML direktorijum.
cp index.html ishrana-i-hidratacija.html kampovi-i-manifestacije.html naruci-plan.html uspesna-uplata.html moji-planovi.html pravni-uslovi.html demo-portal.html demo-portal.js payments-language.js sitemap.xml /home/krle/html/crnakobra/

# 4. Kada se menja knjiga, kopiraj ceo folder sa videima, QR kodovima i stranicom knjige.
cp -a 'knjiga prvo izdanje' /home/krle/html/crnakobra/

# 5. Za objavu samo knjige potrebne su ove dve komande.
cp index.html /home/krle/html/crnakobra/index.html
cp -a 'knjiga prvo izdanje' /home/krle/html/crnakobra/
```

### Objavljivanje današnje aktivnosti i arhive

Kada se menja „Današnja aktivnost”, na server treba kopirati i `index.html` i ceo folder `Arhiva/`, jer se fotografije aktivnosti čuvaju po datumima unutar tog foldera. Posle commita i slanja izmena na `main`, na serveru pokrenuti:

```bash
cd /home/krle/apps/crnakobra/crnakobra
git pull --ff-only origin main
cp index.html /home/krle/html/crnakobra/index.html
cp -a Arhiva /home/krle/html/crnakobra/
```

Na primer, fotografija aktivnosti od 8. septembra 2026. nalazi se u `Arhiva/2026-09-08/`, a prethodna od 13. avgusta 2026. u `Arhiva/2026-08-13/`. Nazive ovih foldera zadržavati u formatu `GGGG-MM-DD`, da aktivnosti ostanu poredane po datumu.

Za pojedinačnu izmenu početne strane može se koristiti i:

```bash
cp index.html /home/krle/html/crnakobra/index.html
```

Napomena: nije potrebno menjati produkcioni radni direktorijum na granu `develop`; javni statični sajt se ažurira sa grane `main`.
