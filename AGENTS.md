# Crna Kobra — kontekst za agente

## Projekat i trenutno stanje

- Ovo je uglavnom statičan sajt Instituta za borilačke veštine i sportove Crna Kobra (Pirot): HTML, ugrađeni CSS i malo JavaScript-a; nema Node paketa ni build koraka.
- Primarni sadržaj je na srpskom latinicom. `payments-language.js` dodaje LAT/ĆIR/EN prekidač na stranicama vezanim za planove i naplatu.
- `main` je glavna grana. Postoji i udaljena `develop` grana, trenutno na istom commitu kao `main`.
- Poslednje izmene početne strane dodale su najavu kampa i detalje Warrior Camp-a. Pre izmene sadržaja pogledati odgovarajući HTML, jer su stilovi najčešće ugrađeni u isti fajl.

## Mapa repozitorijuma

- `index.html` — početna strana.
- `kampovi-i-manifestacije.html` — kampovi i manifestacije.
- `ishrana-i-hidratacija.html` — sadržaj o ishrani/hidrataciji.
- `naruci-plan.html`, `uspesna-uplata.html`, `moji-planovi.html`, `pravni-uslovi.html` — trenutni ručni tok porudžbine i pravne informacije.
- `demo-portal.html` i `demo-portal.js` — odvojeni, neindeksirani demo korisničkog prostora; nije stvarna naplata.
- `payments-language.js` — klijentska transliteracija i delimični prevod navedenih payment stranica.
- `payments-api/` — mali Go API samo za demonstraciju budućeg payment toka.
- `slike/` — slike i video materijali; postojeće relativne putanje ne menjati bez provere svih referenci.
- `deploy/` — šabloni za Nginx, systemd i Docker demo postavljanje.

## Naplata: važne granice

- Trenutni javni sajt **ne obrađuje kartice**. Porudžbina se potvrđuje ručno, zatim se koristi poslovni račun/IPS QR ili PayPal faktura; plan se isporučuje nakon proverene uplate.
- `payments-api` je samo lokalni/demo prototip: podaci su u memoriji, magic link se prikazuje u interfejsu, checkout i webhook su simulirani, a PDF je generisan primer. Ne uvoditi ga kao produkcionu naplatu.
- Demo pokretati samo sa `DEMO_MODE=true`. Za server je predviđen poseban poddomen `demo.crnakobra.com`, Nginx Basic Auth i tajna `DEMO_SECRET` van repozitorijuma.
- Pre stvarne online naplate potrebni su pravi payment provajder/webhook, e-mail servis, baza korisnika i porudžbina, bezbedno čuvanje tajni, kao i potvrđeni poslovni i pravni podaci. Ne predstavljati demo kao produkciono rešenje.
- Tekst na `pravni-uslovi.html` je radna verzija i sam navodi da zahteva proveru sa knjigovođom/pravnikom pre pokretanja naplate.

## Lokalni rad i provere

- Statične izmene: otvoriti relevantan HTML u pregledaču; nema kompilacije.
- Demo API: `cd payments-api && DEMO_MODE=true GOCACHE=/tmp/crna-kobra-go-cache go run .`, potom otvoriti `http://localhost:8080/demo-portal.html`.
- Docker demo: iz korena koristiti `docker compose -f docker-compose.demo.yml up -d --build`; `.env.demo` je lokalna tajna i ne treba je commitovati.
- Posle izmene `payments-language.js` proveriti LAT, ĆIR i EN prikaz na svakoj stranici koju skripta menja. Prevod je zasnovan na tačnom podudaranju teksta, pa promena teksta može zahtevati izmenu rečnika.
- Posle izmene linkova, slika ili navigacije proveriti relativne putanje i mobilni prikaz.

## Postavljanje statičkog sajta

- Server klon je prema `README.md` u `/home/krle/apps/crnakobra/crnakobra`.
- Uobičajen tok je `git pull`, pa kopiranje promenjenih statičkih fajlova u `/home/krle/html/crnakobra/` (taj direktorijum služi sadržaj). Za samo demo portal dovoljan je `cp demo-portal.html /home/krle/html/crnakobra/`.
- Ne commitovati stvarne tajne, `.env` fajlove, pristupne podatke ili TLS/Basic-Auth materijal.

## Smernice za izmene

- Čuvati postojeći vizuelni jezik: tamna `#151512`, svetla pozadina, crveni akcenat `#cf2f24`, zlatni `#d8b16b`, Georgia za naslove i Arial za tekst.
- Ne menjati medicinske ili pravne tvrdnje bez izričitog zahteva i stručne provere; planovi su informativni, ne medicinski savet.
- Ne menjati tekst koji kaže da se plaćanje potvrđuje ručno, osim kada korisnik izričito prelazi na stvarno, provereno payment rešenje.
