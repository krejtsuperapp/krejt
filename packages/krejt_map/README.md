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

## Pse pamje e palëvizshme dhe jo SDK vendase

SDK-ja vendase e Mapbox-it kërkon një token shkarkimi te makina dhe te CI-ja, dhe atë e vendos
vetëm pronari i llogarisë. Static Images API-ja jep të njëjtat rrugë me një kërkesë HTTPS, pa asnjë
hap vendas — ndaj harta punon që sot. Kur të vendoset ofruesi përfundimtar, SDK-ja hyn si një
zbatim i dytë pas së njëjtës ndërfaqe.

## Kostoja

Çdo pamje faturohet. Shoferi lëviz vazhdimisht, ndaj `KMap.settle` e mban pamjen e mëparshme
derisa dikush të ketë lëvizur mbi njëzet e pesë metra — pa këtë, çdo pyetje e serverit do të
blinte një pamje të re për një ndryshim që syri nuk e dallon.
