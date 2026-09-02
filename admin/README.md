# KREJT — Paneli i Operacioneve

Ndërfaqja e stafit mbi të njëjtin API si aplikacionet. Next.js, pa gjendje të vetën:
çdo e dhënë vjen nga serveri dhe çdo e drejtë zbatohet atje.

## Si e nis

```bash
npm install
npm run dev
```

Adresa e API-së merret nga `KREJT_API_BASE_URL` (shih `.env.example`).

## Si është ndërtuar

**Token-at nuk e prekin kurrë shfletuesin.** Kyçja kalon nga `/api/auth`, që i vendos në cookie
`httpOnly`; çdo thirrje tjetër shkon te `/api/krejt/...`, një proxy që ia vë bearer-in kërkesës dhe
e rifreskon kur skadon. Një XSS i vetëm nuk dorëzon dot një llogari me të drejta bllokimi.

**Menyja ndjek kapacitetet.** Secili sheh vetëm atë që bën dot; serveri i zbaton gjithsesi, ndaj
fshehja është vetëm mirësjellje ndaj përdoruesit, jo mbrojtje.

**Paraja mbetet numër i plotë në cent**, si kudo tjetër në KREJT.

## Kontrollet

```bash
npm run lint
npm run typecheck
npm test
npm run build
```
