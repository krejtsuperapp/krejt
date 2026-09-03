# Prova nga fillimi në fund (dev)

Skripte pa varësi, vetëm Node 22+, kundrejt një mjedisi me numrat e provës (kod fiks `111111`).
Nuk përmbajnë asnjë sekret: numrat e provës kyçen pa SMS dhe punojnë vetëm në development.

| Skripti | Çfarë provon |
| --- | --- |
| `driver-setup.mjs` | shoferi i provës hyn, administratori e regjistron nga zyra dhe e aprovon |
| `ride.mjs` | udhëtim i plotë: porosi → ofertë → pranim → mbërritje → nisje me kod → përfundim → vlerësim |
| `food.mjs` | porosi ushqimi: tregtari regjistrohet dhe aktivizohet, menyja krijohet, porosi → pranim → përgatitje → gati → kurieri merr me kod → dorëzon |

```bash
node tests/e2e/driver-setup.mjs   # një herë për mjedis
BASE=https://dev.krejt.app node tests/e2e/ride.mjs
BASE=https://dev.krejt.app node tests/e2e/food.mjs
```

CI-ja i ekzekuton `ride.mjs` dhe `food.mjs` pas çdo deploy-i në dev (puna `smoke-dev`): nëse rrjedha e udhëtimit
thyhet, deploy-i shënohet i kuq edhe pse kontejneri u ngrit.

Këto skripte zbuluan tre defekte të vërteta kontrate te aplikacionet (pozicioni si grup mostrash,
`code` te nisja, etiketat e vlerësimit) që asnjë test njësie nuk i kapte.
