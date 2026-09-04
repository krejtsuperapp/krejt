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

  /// Kanali i gjallë: një lidhje për sesion, e hapur në herën e parë që një ekran e kërkon.
  RealtimeClient? _realtime;
  RealtimeClient get realtime => _realtime ??= RealtimeClient(api);

  /// Njoftimet push: ndizen pas kyçjes, vetëm nëse konfigurimi i Firebase-it është dhënë.
  PushService? _push;
  PushService get push => _push ??= PushService(api);

  BootPhase phase = BootPhase.starting;
  PublicConfig config = PublicConfig.fallback();
  Me? me;
  Ride? activeRide;
  Order? activeOrder;
  Parcel? activeParcel;
  List<Ride> recentRides = const [];
  String locale = 'sq';
  ApiError? bootError;

  bool get isSignedIn => me != null;
  bool get hasActiveRide => activeRide != null;

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
      if (me!.locale != locale) {
        // Serveri mban gjuhën e llogarisë; pajisja e ndjek atë pas kyçjes.
        await setLocale(me!.locale, sync: false);
      }
      phase = BootPhase.ready;
      unawaited(refreshRides());
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

  /// Ndërrim i përkohshëm në hyrje: pajisja e ndjek, por asgjë nuk ruhet derisa përdoruesi të fillojë.
  void previewLocale(String value) {
    locale = value;
    api.locale = value;
    notifyListeners();
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

  /// Historiku i fundit dhe udhëtimi aktiv vijnë nga i njëjti burim: serveri i rendit
  /// nga më i riu, ndaj i pari mjafton për të ditur nëse diçka është ende në rrjedhë (§20).
  Future<void> refreshRides() async {
    try {
      final rides = await api.rideHistory(limit: 10);
      recentRides = rides;
      Ride? active;
      for (final r in rides) {
        if (r.isActive) {
          active = r;
          break;
        }
      }
      activeRide = active;
      notifyListeners();
    } on ApiError {
      // Pamja e vjetër mbetet; rifreskohet në veprimin e radhës.
    }
  }

  /// Porosia aktive vjen nga historiku njësoj si udhëtimi: serveri i rendit nga më i riu.
  Future<void> refreshOrders() async {
    try {
      final orders = await api.orderHistory(limit: 5);
      Order? active;
      for (final o in orders) {
        if (o.isActive) {
          active = o;
          break;
        }
      }
      activeOrder = active;
      notifyListeners();
    } on ApiError {
      // Pamja e vjetër mbetet; rifreskohet në veprimin e radhës.
    }
  }

  Future<void> refreshParcels() async {
    try {
      activeParcel = await api.activeParcel();
      notifyListeners();
    } on ApiError {
      // Pamja e vjetër mbetet; rifreskohet në veprimin e radhës.
    }
  }

  Future<void> refreshHome() async {
    await Future.wait([refreshMe(), refreshRides(), refreshOrders(), refreshParcels()]);
  }

  /// Ruajtja e profilit kthen përdoruesin e ri nga serveri; ekranet e shohin menjëherë.
  Future<void> saveProfile({String? fullName, String? email}) async {
    me = await api.updateProfile(fullName: fullName, email: email);
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

  /// Njoftimi (§47) është shtytje: gjendja merret nga serveri, që modeli të mbetet një.
  void _onPush(Map<String, dynamic> data, {required bool opened}) {
    unawaited(refreshRides());
  }

  Future<void> signOut() async {
    await push.stop();
    await api.logout();
    me = null;
    activeRide = null;
    activeOrder = null;
    activeParcel = null;
    recentRides = const [];
    phase = BootPhase.signedOut;
    notifyListeners();
  }

  void _onSessionExpired() {
    me = null;
    activeRide = null;
    activeOrder = null;
    recentRides = const [];
    phase = BootPhase.signedOut;
    notifyListeners();
  }
}
