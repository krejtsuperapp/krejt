import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Magazina ku rrinë token-at. Në pajisje është Keystore ose Keychain; testet japin një të tyren,
/// që sjellja e sesionit të provohet pa pasur nevojë për një pajisje të vërtetë.
abstract class SecureStore {
  Future<String?> read(String key);
  Future<void> write(String key, String value);
  Future<void> delete(String key);
}

class _PlatformStore implements SecureStore {
  const _PlatformStore();

  static const _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
    iOptions: IOSOptions(accessibility: KeychainAccessibility.first_unlock),
  );

  @override
  Future<String?> read(String key) => _storage.read(key: key);

  @override
  Future<void> write(String key, String value) => _storage.write(key: key, value: value);

  @override
  Future<void> delete(String key) => _storage.delete(key: key);
}

/// Magazinë në kujtesë, vetëm për teste.
class MemoryStore implements SecureStore {
  final Map<String, String> _values = {};

  @override
  Future<String?> read(String key) async => _values[key];

  @override
  Future<void> write(String key, String value) async => _values[key] = value;

  @override
  Future<void> delete(String key) async => _values.remove(key);
}

/// Sesioni i pajisjes: access token jetëshkurtër dhe refresh token me rotacion (§53).
/// Token-at ruhen vetëm në magazinën e sigurt të sistemit — kurrë në SharedPreferences apo në log.
class Session {
  Session({SecureStore? store}) : _store = store ?? const _PlatformStore();

  static const _key = 'krejt.session.v1';
  static const _deviceKey = 'krejt.device.v1';

  final SecureStore _store;
  final _changes = StreamController<Session>.broadcast();

  String? _access;
  String? _refresh;
  DateTime? _expiresAt;
  String? _userId;
  String? _deviceId;

  Stream<Session> get changes => _changes.stream;

  /// Identifikuesi i kësaj pajisjeje. Serveri e kërkon te kyçja dhe e lidh me sesionin, që
  /// përdoruesi t'i shohë pajisjet e veta dhe të heqë njërën pa i prekur të tjerat (§53).
  /// Krijohet një herë dhe rri sa vetë instalimi; nuk është sekret dhe nuk identifikon njeri.
  String get deviceId => _deviceId ??= _newDeviceId();

  String? get accessToken => _access;
  String? get refreshToken => _refresh;
  String? get userId => _userId;
  bool get isAuthenticated => _refresh != null;

  /// A ka skaduar (ose po skadon brenda 60 s) access token-i?
  bool get needsRefresh {
    if (_access == null) return _refresh != null;
    final exp = _expiresAt;
    if (exp == null) return false;
    return DateTime.now().isAfter(exp.subtract(const Duration(seconds: 60)));
  }

  Future<void> load() async {
    final stored = await _store.read(_deviceKey);
    if (stored == null) {
      await _store.write(_deviceKey, deviceId);
    } else {
      _deviceId = stored;
    }

    final raw = await _store.read(_key);
    if (raw == null) return;
    try {
      final m = jsonDecode(raw) as Map<String, dynamic>;
      _access = m['a'] as String?;
      _refresh = m['r'] as String?;
      _userId = m['u'] as String?;
      final exp = m['e'] as String?;
      _expiresAt = exp == null ? null : DateTime.tryParse(exp);
    } catch (_) {
      await clear();
    }
  }

  Future<void> save({
    required String accessToken,
    required String refreshToken,
    DateTime? expiresAt,
    String? userId,
  }) async {
    _access = accessToken;
    _refresh = refreshToken;
    _expiresAt = expiresAt;
    _userId = userId ?? _userId;
    await _store.write(
      _key,
      jsonEncode({'a': _access, 'r': _refresh, 'e': _expiresAt?.toIso8601String(), 'u': _userId}),
    );
    _changes.add(this);
  }

  /// Dalja fshin sesionin, jo pajisjen: i njëjti telefon mbetet i njëjti te lista e pajisjeve.
  Future<void> clear() async {
    _access = null;
    _refresh = null;
    _expiresAt = null;
    _userId = null;
    await _store.delete(_key);
    _changes.add(this);
  }

  void dispose() => _changes.close();

  static String _newDeviceId() {
    final r = Random.secure();
    final b = List<int>.generate(16, (_) => r.nextInt(256));
    return b.map((x) => x.toRadixString(16).padLeft(2, '0')).join();
  }
}
