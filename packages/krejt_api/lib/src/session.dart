import 'dart:async';
import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Sesioni i pajisjes: access token jetëshkurtër dhe refresh token me rotacion (§53).
/// Token-at ruhen vetëm në magazinën e sigurt të sistemit — kurrë në SharedPreferences apo në log.
class Session {
  Session({FlutterSecureStorage? storage})
    : _storage =
          storage ??
          const FlutterSecureStorage(
            aOptions: AndroidOptions(encryptedSharedPreferences: true),
            iOptions: IOSOptions(accessibility: KeychainAccessibility.first_unlock),
          );

  static const _key = 'krejt.session.v1';

  final FlutterSecureStorage _storage;
  final _changes = StreamController<Session>.broadcast();

  String? _access;
  String? _refresh;
  DateTime? _expiresAt;
  String? _userId;

  Stream<Session> get changes => _changes.stream;
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
    final raw = await _storage.read(key: _key);
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
    await _storage.write(
      key: _key,
      value: jsonEncode({
        'a': _access,
        'r': _refresh,
        'e': _expiresAt?.toIso8601String(),
        'u': _userId,
      }),
    );
    _changes.add(this);
  }

  Future<void> clear() async {
    _access = null;
    _refresh = null;
    _expiresAt = null;
    _userId = null;
    await _storage.delete(key: _key);
    _changes.add(this);
  }

  void dispose() => _changes.close();
}
