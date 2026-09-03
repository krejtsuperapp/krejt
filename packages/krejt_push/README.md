# krejt_push

Njoftimet push të KREJT-it (§47), mbi Firebase Cloud Messaging. Aplikacionet varen vetëm nga
`PushService`: ai regjistron token-in e pajisjes te serveri ynë, e rifreskon kur ndryshon, e heq
në dalje, dhe ia kalon aplikacionit njoftimin e prekur që të hapë ekranin e duhur.

## Konfigurimi

Katër vlera nga `google-services.json` i projektit Firebase, me `--dart-define`
(ose te `apps/dart-defines.json`):

```
KREJT_FIREBASE_API_KEY
KREJT_FIREBASE_APP_ID
KREJT_FIREBASE_SENDER_ID
KREJT_FIREBASE_PROJECT_ID
```

Janë vlera publike të klientit, jo sekrete — por nuk qëndrojnë te kodi, që dev-i dhe prodhimi të
mos ngatërrohen. Pa to, push-i thjesht nuk ndizet dhe aplikacioni punon njësoj: ngjarjet vijnë
nga kanali i gjallë.

Firebase-i inicializohet me këto vlera drejtpërdrejt, ndaj `google-services.json` nuk ka nevojë
të shtohet te projekti Android.

## Sjellja

- `start()` pas kyçjes: leja, token-i, `POST /notifications/push-token`. Idempotent; çdo gabim
  përpihet — push-i nuk e ndal kurrë kyçjen.
- `stop()` në dalje: `DELETE /notifications/push-token`, para se sesioni të mbyllet.
- Njoftimi i prekur (ose i mbërritur në plan të parë) i kalohet aplikacionit si `data`; ai e
  trajton si shtytje dhe e merr gjendjen nga serveri, që modeli të mbetet një.
