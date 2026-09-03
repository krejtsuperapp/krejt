# krejt_map

Harta e KREJT-it. Ekranet varen vetëm nga `KMap`; ofruesi qëndron pas saj, ndaj vendimi mes
Mapbox-it dhe alternativave mbetet i hapur dhe ndërrimi nuk i prek ata.

## Si zgjidhet ofruesi

Njësoj si te serveri: një emër vendos, dhe pa çelës bihet te varianti vendor.

```
--dart-define=KREJT_MAPBOX_TOKEN=pk.…        # çelësi publik i Mapbox-it
--dart-define=KREJT_MAP_PROVIDER=mapbox      # jo i detyrueshëm; çelësi vetëm mjafton
--dart-define=KREJT_MAPBOX_STYLE=mapbox/dark-v11
```

Pa çelës, harta vizatohet vendorisht: pozicionet janë të vërteta dhe përpjesëtimet mes tyre
ruhen, por rrugët nuk vizatohen — dhe kjo shkruhet mbi pamje, që askush të mos e marrë për hartë.
E njëjta pamje shfaqet edhe kur rrjeti mungon ose çelësi refuzohet, që ekrani të mbetet i
përdorshëm në vend që të mbetet një kuti bosh.

## Çelësi

Vjen gjithmonë me `--dart-define` gjatë ndërtimit; asnjë çelës nuk qëndron te kodi. Përdor një
token **publik** (`pk.`), të kufizuar me URL te paneli i Mapbox-it. Token-at sekretë (`sk.`) nuk
hyjnë kurrë në një aplikacion që shpërndahet.

## Token-i i shkarkimit

SDK-ja e Mapbox-it nuk shkarkohet publikisht: Gradle-ja dhe CocoaPods-i vërtetohen te Mapbox-i
**në kohën e ndërtimit**. Kjo kërkon një token sekret me të drejtën `DOWNLOADS:READ`, dhe vetëm atë —
asnjë të drejtë publike, sepse token-i jeton te shumë makina dhe te CI-ja.

Android: një rresht te `~/.gradle/gradle.properties` (jashtë repo-s):

```
MAPBOX_DOWNLOADS_TOKEN=sk.…
```

iOS: e njëjta gjë te `~/.netrc`.

CI: sekreti `MAPBOX_DOWNLOADS_TOKEN`, që workflow-i e shkruan te dosja e Gradle-së para ndërtimit.

Pa këtë token ndërtimi dështon — jo aplikacioni, por vetë kompilimi.
