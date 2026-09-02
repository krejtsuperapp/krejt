# KREJT — Paneli i partnerit

Ndërfaqja e vendit që pranon porositë: radha e kuzhinës, menuja dhe cilësimet e ditës.
Ndërtuar për një tablet kuzhine, jo për një tavolinë zyre.

## Si e nis

```bash
npm install
npm run dev
```

Adresa e API-së merret nga `KREJT_API_BASE_URL` (shih `.env.example`).

## Vendimet

**Prekja para dendësisë.** Objektivi minimal është 56 px dhe teksti lexohet nga një hap larg,
sepse kuzhina prek me duar të zëna dhe pa shikuar mirë.

**Gjendja shihet para se të lexohet.** Ngjyra e skajit të kartës e thotë hapin; porosia që ka
pritur mbi njëzet minuta bëhet e kuqe pa u kërkuar.

**Një sinjal kur vjen porosi e re**, sepse askush nuk rri duke parë ekranin gjatë punës. Sinjali
krijohet me Web Audio, ndaj nuk varet nga asnjë skedar, dhe bie vetëm kur numri i porosive të reja
rritet.

**Token-at nuk e prekin kurrë shfletuesin.** Si te paneli i Operacioneve: kyçja kalon nga
`/api/auth` që i vendos në cookie `httpOnly`, dhe çdo thirrje tjetër shkon nga një proxy i vetëm.

**Çmimet nuk ndryshohen këtu.** Ato i vendos marrëveshja, që klienti të mos gjejë çmim tjetër nga
ai që pa te menuja.

## Kontrollet

```bash
npm run lint
npm run typecheck
npm test
npm run build
```
