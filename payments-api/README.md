# Crna Kobra — demo naplate

Ovo je lokalna simulacija budućeg payment API-ja. Ne šalje e-mail, ne naplaćuje novac i ne sme se postaviti u produkciju.

## Pokretanje

```bash
cd payments-api
DEMO_MODE=true GOCACHE=/tmp/crna-kobra-go-cache go run .
```

Zatim otvorite `http://localhost:8080/demo-portal.html`.

## Tok demonstracije

1. Kupac unese e-mail; demo prikaže magic link umesto slanja e-maila.
2. Magic link postavlja bezbedni session kolačić i otvara korisnički prostor.
3. Kupac bira gotov mesečni ili personalizovani plan.
4. Dugme „Simuliraj plaćanje” kreira porudžbinu i internu potpisanu webhook potvrdu.
5. Gotov plan se otključava kroz zaštićeni PDF endpoint; personalizovanom planu se postavlja status i prikazuje upitnik.

U stvarnoj integraciji demonstracioni checkout se uklanja, a bankin/PayPal webhook poziva isti deo koji dodeljuje pravo pristupa. Magic link se šalje preko stvarnog e-mail provajdera, a korisnici, porudžbine i prava pristupa čuvaju se u bazi podataka, ne u memoriji.

## Zaštićeno postavljanje na server

Šabloni za Nginx, systemd i promenljive okruženja nalaze se u `../deploy/`. Demo se postavlja na poseban poddomen i iza Basic Auth zaštite. Ne postavljati ga kao javnu naplatu niti koristiti `DEMO_MODE=true` na produkcionom payment API-ju.
