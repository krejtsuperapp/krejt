/// Njoftimet push (§47). Serveri di t'i dërgojë me FCM; kjo paketë e regjistron token-in e
/// pajisjes te serveri dhe ia kalon aplikacionit njoftimin e prekur, që të hapë ekranin e duhur.
///
/// Konfigurimi i Firebase-it vjen me `--dart-define` — janë vlera publike të klientit, jo
/// sekrete, por nuk qëndrojnë te kodi që dy mjedise (dev/prod) të mos ngatërrohen. Pa to, push-i
/// thjesht nuk ndizet dhe aplikacioni punon njësoj: ngjarjet vijnë nga kanali i gjallë.
library;

import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:krejt_api/krejt_api.dart';

class PushConfig {
  const PushConfig({
    required this.apiKey,
    required this.appId,
    required this.senderId,
    required this.projectId,
  });

  final String apiKey;
  final String appId;
  final String senderId;
  final String projectId;

  /// `KREJT_FIREBASE_API_KEY`, `KREJT_FIREBASE_APP_ID`, `KREJT_FIREBASE_SENDER_ID`,
  /// `KREJT_FIREBASE_PROJECT_ID`. Katër vlera nga `google-services.json` i projektit.
  factory PushConfig.fromEnv({
    String apiKey = const String.fromEnvironment('KREJT_FIREBASE_API_KEY'),
    String appId = const String.fromEnvironment('KREJT_FIREBASE_APP_ID'),
    String senderId = const String.fromEnvironment('KREJT_FIREBASE_SENDER_ID'),
    String projectId = const String.fromEnvironment('KREJT_FIREBASE_PROJECT_ID'),
  }) => PushConfig(apiKey: apiKey, appId: appId, senderId: senderId, projectId: projectId);

  /// Ndizet vetëm kur të katër vlerat janë aty: gjysma e konfigurimit është më keq se asnjë.
  bool get enabled =>
      apiKey.isNotEmpty && appId.isNotEmpty && senderId.isNotEmpty && projectId.isNotEmpty;

  FirebaseOptions get options => FirebaseOptions(
    apiKey: apiKey,
    appId: appId,
    messagingSenderId: senderId,
    projectId: projectId,
  );
}

/// Ngjarja që i kalohet aplikacionit kur përdoruesi prek një njoftim (ose kur vjen në plan
/// të parë). `data` është ajo që dërgon serveri: lloji dhe identifikuesit për rrugëzim.
typedef PushHandler = void Function(Map<String, dynamic> data, {required bool opened});

class PushService {
  PushService(this.api, {PushConfig? config}) : config = config ?? PushConfig.fromEnv();

  final KrejtApi api;
  final PushConfig config;

  StreamSubscription<String>? _refresh;
  StreamSubscription<RemoteMessage>? _foreground;
  StreamSubscription<RemoteMessage>? _opened;
  bool _started = false;

  /// Nis pas kyçjes: leja, token-i, regjistrimi te serveri. Idempotent — thirrja e dytë nuk bën
  /// asgjë. Çdo gabim përpihet: push-i nuk duhet ta ndalë kurrë kyçjen.
  Future<void> start({required PushHandler onMessage}) async {
    if (_started || !config.enabled) return;
    _started = true;
    try {
      if (Firebase.apps.isEmpty) await Firebase.initializeApp(options: config.options);
      final fm = FirebaseMessaging.instance;
      await fm.requestPermission();

      final token = await fm.getToken();
      if (token != null) await _register(token);
      _refresh = fm.onTokenRefresh.listen(_register);

      _foreground = FirebaseMessaging.onMessage.listen((m) => onMessage(_data(m), opened: false));
      _opened = FirebaseMessaging.onMessageOpenedApp.listen(
        (m) => onMessage(_data(m), opened: true),
      );
      // Aplikacioni u hap nga një njoftim ndërsa ishte i mbyllur: e njëjta rrugë.
      final initial = await fm.getInitialMessage();
      if (initial != null) onMessage(_data(initial), opened: true);
    } catch (_) {
      _started = false;
    }
  }

  /// Në dalje: token-i hiqet te serveri që njoftimet e llogarisë të mos vijnë te kjo pajisje.
  Future<void> stop() async {
    await _refresh?.cancel();
    await _foreground?.cancel();
    await _opened?.cancel();
    _refresh = _foreground = _opened = null;
    if (!_started) return;
    _started = false;
    try {
      final token = await FirebaseMessaging.instance.getToken();
      if (token != null) await api.removePushToken(token);
    } catch (_) {
      // Pa rrjet, serveri e pastron vetë kur dërgesa e radhës dështon.
    }
  }

  Future<void> _register(String token) async {
    try {
      await api.registerPushToken(token: token, platform: api.config.platform, locale: api.locale);
    } on ApiError {
      // Rifreskimi i radhës e riprovon; asnjë gjendje nuk humbet.
    }
  }

  static Map<String, dynamic> _data(RemoteMessage m) => Map<String, dynamic>.from(m.data);
}
