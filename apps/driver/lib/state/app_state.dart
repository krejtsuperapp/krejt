import 'dart:async';
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_push/krejt_push.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../env.dart';

enum BootPhase { starting, needsLanguage, signedOut, ready, blocked, failed }

/// Gjendja e përbashkët e aplikacionit: sesioni, konfigurimi publik, përdoruesi dhe gjuha.
/// Çdo ekran e lexon këtu; asnjë ekran nuk e thërret API-në për këto vetë.
class AppState extends ChangeNotifier {
  AppState() {
    _session = Session();
    api = KrejtApi(
      config: ApiConfig(
        baseUrl: Env.apiBaseUrl,
        appId: Env.appId,
        platform: _platform(),
        appVersion: Env.appVersion,
      ),
      session: _session,
    );
    api.onSessionExpired = _onSessionExpired;
  }

  static const _prefLocale = 'krejt.locale';

  late final Session _session;
  late final KrejtApi api;

  /// Kanali i gjallë: një lidhje për sesion, e hapur në herën e parë që turni e kërkon.
  RealtimeClient? _realtime;
  RealtimeClient get realtime => _realtime ??= RealtimeClient(api);

  /// Njoftimet push: ndizen pas kyçjes, vetëm nëse konfigurimi i Firebase-it është dhënë.
  PushService? _push;
  PushService get push => _push ??= PushService(api);

  BootPhase phase = BootPhase.starting;
  PublicConfig config = PublicConfig.fallback();
  Me? me;
  DriverProfile? driver;

  /// Shifrat e ditës për ballinën (fitimet, udhëtimet); vijnë të llogaritura nga serveri.
  Earnings? earnings;
  bool online = false;
  String locale = 'sq';
  ApiError? bootError;

  bool get isSignedIn => me != null;

  /// Shoferi hyn në punë vetëm pasi profili aprovohet dhe dokumentet janë në rregull (§27).
  bool get canGoOnline => driver?.canGoOnline == true;

  static String _platform() {
    if (kIsWeb) return 'web';
    if (Platform.isIOS) return 'ios';
    if (Platform.isAndroid) return 'android';
    return 'other';
  }

  /// Nisja: gjuha e ruajtur, sesioni, konfigurimi publik dhe pastaj profili.
  Future<void> boot() async {
    phase = BootPhase.starting;
    bootError = null;
    notifyListeners();

    final prefs = await SharedPreferences.getInstance();
    final saved = prefs.getString(_prefLocale);
    final hasLanguage = saved != null;
    if (hasLanguage) {
      locale = saved;
      api.locale = saved;
    }

    await _session.load();

    try {
      config = await api.fetchConfig();
    } on ApiError catch (e) {
      // Pa konfigurim vazhdojmë me vlerat e paracaktuara; bllokojmë vetëm nëse edhe profili dështon.
      bootError = e;
    }

    if (config.blocksApp) {
      phase = BootPhase.blocked;
      notifyListeners();
      return;
    }

    if (!hasLanguage) {
      phase = BootPhase.needsLanguage;
      notifyListeners();
      return;
    }

    await _loadMe();
  }

  Future<void> _loadMe() async {
    if (!_session.isAuthenticated) {
      me = null;
      phase = BootPhase.signedOut;
      notifyListeners();
      return;
    }
    try {
      me = await api.me();
      await _loadDriver();
      if (me!.locale != locale) {
        // Serveri mban gjuhën e llogarisë; pajisja e ndjek atë pas kyçjes.
        await setLocale(me!.locale, sync: false);
      }
      phase = BootPhase.ready;
      unawaited(push.start(onMessage: _onPush));
    } on ApiError catch (e) {
      if (e.isUnauthorized) {
        me = null;
        phase = BootPhase.signedOut;
      } else {
        bootError = e;
        phase = BootPhase.failed;
      }
    }
    notifyListeners();
  }

  /// Pas aplikimit: profili i ri merret nga serveri që ekrani i punës ta shohë menjëherë.
  Future<void> refreshDriver() async {
    await _loadDriver();
    notifyListeners();
  }

  Future<void> _loadDriver() async {
    try {
      driver = await api.driverProfile();
    } on ApiError catch (e) {
      // 404 do të thotë se ky përdorues ende nuk ka aplikuar si shofer.
      if (!e.isNotFound) rethrow;
      driver = null;
    }
    if (driver == null) {
      earnings = null;
      return;
    }
    try {
      earnings = await api.earnings();
    } on ApiError {
      // Shifrat e ditës janë ndihmëse: pa to ballina mbetet e përdorshme.
    }
  }

  /// Hyrja dhe dalja nga puna. Gjendja vendoset nga serveri; ky flamur vetëm e pasqyron.
  Future<void> setOnline(bool value) async {
    if (value && !canGoOnline) return;
    if (value) {
      await api.goOnline(driver!.categories.map((c) => c.name).toList());
    } else {
      await api.goOffline();
    }
    online = value;
    notifyListeners();
  }

  Future<void> refreshDriver() async {
    await _loadDriver();
    notifyListeners();
  }

  Future<void> setLocale(String value, {bool sync = true}) async {
    locale = value;
    api.locale = value;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefLocale, value);
    notifyListeners();
    if (sync && _session.isAuthenticated) {
      try {
        me = await api.updateProfile(locale: value);
        notifyListeners();
      } on ApiError {
        // Gjuha lokale mbetet; serveri sinkronizohet në hyrjen e radhës.
      }
    }
  }

  /// Thirret pas zgjedhjes së gjuhës në ekranin e parë.
  Future<void> completeLanguage(String value) async {
    await setLocale(value, sync: false);
    await _loadMe();
  }

  Future<void> onSignedIn(Me user) async {
    me = user;
    phase = BootPhase.ready;
    notifyListeners();
  }

  Future<void> refreshMe() async {
    try {
      me = await api.me();
      notifyListeners();
    } on ApiError {
      // Pamja e vjetër mbetet; do të rifreskohet në veprimin e radhës.
    }
  }

  /// Njoftimi (§47) është shtytje: profili dhe turni rifreskohen nga serveri.
  void _onPush(Map<String, dynamic> data, {required bool opened}) {
    unawaited(_loadDriver().then((_) => notifyListeners()));
  }

  Future<void> signOut() async {
    await push.stop();
    await api.logout();
    me = null;
    driver = null;
    online = false;
    phase = BootPhase.signedOut;
    notifyListeners();
  }

  void _onSessionExpired() {
    me = null;
    phase = BootPhase.signedOut;
    notifyListeners();
  }
}
