import 'dart:async';
import 'dart:math';

import 'package:dio/dio.dart';

import 'errors.dart';
import 'models/config.dart';
import 'models/driver.dart';
import 'models/ride.dart';
import 'models/user.dart';
import 'models/wallet.dart';
import 'session.dart';

/// Cilësimet e ndërtimit që shkojnë në çdo kërkesë si header (§48, §69).
class ApiConfig {
  const ApiConfig({
    required this.baseUrl,
    required this.appId,
    required this.platform,
    required this.appVersion,
  });

  final String baseUrl;

  /// `customer` ose `driver`.
  final String appId;

  /// `android` ose `ios`.
  final String platform;
  final String appVersion;
}

/// Gjeneron çelësa idempotence për çdo veprim që krijon para ose udhëtime (§39).
String newIdempotencyKey() {
  final r = Random.secure();
  final b = List<int>.generate(16, (_) => r.nextInt(256));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  final hex = b.map((x) => x.toRadixString(16).padLeft(2, '0')).join();
  return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-${hex.substring(12, 16)}-'
      '${hex.substring(16, 20)}-${hex.substring(20)}';
}

/// Klienti i vetëm i API-së. Çdo gabim del si [ApiError]; asnjë përgjigje e papërpunuar nuk kalon jashtë.
class KrejtApi {
  KrejtApi({required this.config, required this.session, Dio? dio, this.locale = 'sq'})
    : _dio = dio ?? Dio() {
    _dio.options = _dio.options.copyWith(
      baseUrl: config.baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 20),
      sendTimeout: const Duration(seconds: 20),
      validateStatus: (s) => s != null && s < 400,
      headers: {
        'X-App-Id': config.appId,
        'X-App-Platform': config.platform,
        'X-App-Version': config.appVersion,
      },
    );
    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          options.headers['Accept-Language'] = locale;
          if (options.extra['anon'] != true) {
            if (session.needsRefresh) {
              await _refresh();
            }
            final t = session.accessToken;
            if (t != null) options.headers['Authorization'] = 'Bearer $t';
          }
          handler.next(options);
        },
        onError: (e, handler) async {
          final retriable =
              e.response?.statusCode == 401 &&
              e.requestOptions.extra['retried'] != true &&
              e.requestOptions.extra['anon'] != true &&
              session.refreshToken != null;
          if (!retriable) return handler.next(e);
          final ok = await _refresh();
          if (!ok) return handler.next(e);
          final o = e.requestOptions;
          o.extra['retried'] = true;
          o.headers['Authorization'] = 'Bearer ${session.accessToken}';
          try {
            handler.resolve(await _dio.fetch<dynamic>(o));
          } on DioException catch (err) {
            handler.next(err);
          }
        },
      ),
    );
  }

  final ApiConfig config;
  final Session session;
  final Dio _dio;

  /// Gjuha e kërkesave; ndryshimi ndikon menjëherë te `Accept-Language`.
  String locale;

  /// Thirret kur sesioni nuk rikthehet dot — aplikacioni kthehet te hyrja.
  void Function()? onSessionExpired;

  Future<bool>? _refreshing;

  /// Rifreskim me rotacion, i bashkuar në një kërkesë të vetme edhe kur disa thirrje presin njëherësh.
  Future<bool> _refresh() {
    return _refreshing ??= _doRefresh().whenComplete(() => _refreshing = null);
  }

  Future<bool> _doRefresh() async {
    final rt = session.refreshToken;
    if (rt == null) return false;
    try {
      final res = await _dio.post<dynamic>(
        '/api/v1/auth/token/refresh',
        data: {'refresh_token': rt},
        options: Options(extra: {'anon': true}),
      );
      final pair = TokenPair.fromJson(Map<String, dynamic>.from(res.data as Map));
      await session.save(
        accessToken: pair.accessToken,
        refreshToken: pair.refreshToken,
        expiresAt: pair.expiresAt,
        userId: pair.userId,
      );
      return true;
    } on DioException catch (e) {
      final status = e.response?.statusCode ?? 0;
      if (status == 401 || status == 403) {
        await session.clear();
        onSessionExpired?.call();
      }
      return false;
    }
  }

  // ---------------------------------------------------------------- transport

  Future<Map<String, dynamic>> _get(
    String path, {
    Map<String, dynamic>? query,
    bool anon = false,
  }) async {
    final res = await _send(
      () => _dio.get<dynamic>(
        path,
        queryParameters: query,
        options: Options(extra: {'anon': anon}),
      ),
    );
    return _asMap(res);
  }

  Future<List<Map<String, dynamic>>> _getList(
    String path,
    String key, {
    Map<String, dynamic>? query,
  }) async {
    final m = await _get(path, query: query);
    final items = m[key];
    if (items is! List) return const [];
    return items.map((e) => Map<String, dynamic>.from(e as Map)).toList();
  }

  Future<Map<String, dynamic>> _post(
    String path, {
    Object? body,
    String? idempotencyKey,
    bool anon = false,
  }) async {
    final res = await _send(
      () => _dio.post<dynamic>(
        path,
        data: body,
        options: Options(
          extra: {'anon': anon},
          headers: idempotencyKey == null ? null : {'Idempotency-Key': idempotencyKey},
        ),
      ),
    );
    return _asMap(res);
  }

  Future<Map<String, dynamic>> _patch(String path, {Object? body}) async {
    final res = await _send(() => _dio.patch<dynamic>(path, data: body));
    return _asMap(res);
  }

  Future<void> _delete(String path) async {
    await _send(() => _dio.delete<dynamic>(path));
  }

  Future<Response<dynamic>> _send(Future<Response<dynamic>> Function() run) async {
    try {
      return await run();
    } on DioException catch (e) {
      throw ApiError.fromDio(e);
    } catch (e) {
      throw ApiError.unknown(e);
    }
  }

  Map<String, dynamic> _asMap(Response<dynamic> res) {
    final d = res.data;
    if (d is Map) return Map<String, dynamic>.from(d);
    return const {};
  }

  // ------------------------------------------------------------------- config

  Future<PublicConfig> fetchConfig() async =>
      PublicConfig.fromJson(await _get('/api/v1/config', anon: true));

  // --------------------------------------------------------------------- auth

  /// Kërkon kodin njëpërdorimësh. Serveri kthen të njëjtën përgjigje edhe kur numri nuk ekziston.
  Future<void> requestOtp(String phone) =>
      _post('/api/v1/auth/otp/request', body: {'phone': phone}, anon: true);

  Future<Me> verifyOtp({required String phone, required String code, String? deviceName}) async {
    final j = await _post(
      '/api/v1/auth/otp/verify',
      body: {'phone': phone, 'code': code, 'device_name': ?deviceName},
      anon: true,
    );
    final pair = TokenPair.fromJson(j);
    await session.save(
      accessToken: pair.accessToken,
      refreshToken: pair.refreshToken,
      expiresAt: pair.expiresAt,
      userId: pair.userId,
    );
    return me();
  }

  Future<void> logout() async {
    final rt = session.refreshToken;
    try {
      if (rt != null) await _post('/api/v1/auth/logout', body: {'refresh_token': rt});
    } on ApiError {
      // Dalja lokale ndodh gjithsesi.
    }
    await session.clear();
  }

  Future<void> logoutAll() async {
    try {
      await _post('/api/v1/auth/logout-all');
    } finally {
      await session.clear();
    }
  }

  // -------------------------------------------------------------------- users

  Future<Me> me() async => Me.fromJson(await _get('/api/v1/users/me'));

  Future<Me> updateProfile({String? fullName, String? email, String? locale}) async {
    await _patch(
      '/api/v1/users/me',
      body: {'full_name': ?fullName, 'email': ?email, 'locale': ?locale},
    );
    return me();
  }

  Future<List<Address>> addresses() async {
    final rows = await _getList('/api/v1/users/me/addresses', 'items');
    return rows.map(Address.fromJson).toList();
  }

  Future<Address> addAddress(Address a) async =>
      Address.fromJson(await _post('/api/v1/users/me/addresses', body: a.toJson()));

  Future<void> deleteAddress(String id) => _delete('/api/v1/users/me/addresses/$id');

  Future<List<NotificationPreference>> notificationPreferences() async {
    final rows = await _getList('/api/v1/users/me/notification-preferences', 'items');
    return rows.map(NotificationPreference.fromJson).toList();
  }

  Future<void> updateNotificationPreference(NotificationPreference p) =>
      _patch('/api/v1/users/me/notification-preferences', body: p.toJson());

  Future<List<DeviceSession>> sessions() async {
    final rows = await _getList('/api/v1/users/me/sessions', 'items');
    return rows.map(DeviceSession.fromJson).toList();
  }

  Future<void> revokeSession(String id) => _delete('/api/v1/users/me/sessions/$id');

  // ------------------------------------------------------------ notifications

  Future<List<AppNotification>> notifications({int limit = 30}) async {
    final rows = await _getList('/api/v1/notifications', 'items', query: {'limit': limit});
    return rows.map(AppNotification.fromJson).toList();
  }

  Future<void> markNotificationRead(String id) => _post('/api/v1/notifications/$id/read');

  Future<void> markAllNotificationsRead() => _post('/api/v1/notifications/read-all');

  Future<void> registerPushToken({required String token, required String platform}) =>
      _post('/api/v1/notifications/push-token', body: {'token': token, 'platform': platform});

  // ------------------------------------------------------------------ realtime

  Future<Map<String, dynamic>> realtimeToken() => _post('/api/v1/realtime/token');

  // --------------------------------------------------------------------- rides

  Future<QuoteResult> quoteRide({
    required LatLng pickup,
    required LatLng dropoff,
    String? pickupAddress,
    String? dropoffAddress,
  }) async => QuoteResult.fromJson(
    await _post(
      '/api/v1/rides/quote',
      body: {
        'pickup': pickup.toJson(),
        'dropoff': dropoff.toJson(),
        'pickup_address': ?pickupAddress,
        'dropoff_address': ?dropoffAddress,
      },
    ),
  );

  Future<Ride> requestRide({
    required String quoteId,
    required String paymentMethod,
    String? note,
    String? idempotencyKey,
  }) async => Ride.fromJson(
    await _post(
      '/api/v1/rides',
      body: {
        'quote_id': quoteId,
        'payment_method': paymentMethod,
        if (note != null && note.isNotEmpty) 'note': note,
      },
      idempotencyKey: idempotencyKey ?? newIdempotencyKey(),
    ),
  );

  Future<Ride> ride(String id) async => Ride.fromJson(await _get('/api/v1/rides/$id'));

  Future<List<Ride>> rideHistory({int limit = 20, DateTime? before}) async {
    final rows = await _getList(
      '/api/v1/rides',
      'items',
      query: {'limit': limit, if (before != null) 'before': before.toUtc().toIso8601String()},
    );
    return rows.map(Ride.fromJson).toList();
  }

  Future<Ride> cancelRide(String id, {String? reason}) async =>
      Ride.fromJson(await _post('/api/v1/rides/$id/cancel', body: {'reason': ?reason}));

  Future<PickupToken> pickupQr(String id) async =>
      PickupToken.fromJson(await _get('/api/v1/rides/$id/qr'));

  Future<List<ChatMessage>> rideChat(String id, {DateTime? after}) async {
    final rows = await _getList(
      '/api/v1/rides/$id/chat',
      'items',
      query: {if (after != null) 'after': after.toUtc().toIso8601String()},
    );
    return rows.map(ChatMessage.fromJson).toList();
  }

  Future<ChatMessage> sendRideMessage(String id, String body) async =>
      ChatMessage.fromJson(await _post('/api/v1/rides/$id/chat', body: {'body': body}));

  Future<void> reviewRide(String id, {required int rating, List<String>? tags, String? comment}) =>
      _post(
        '/api/v1/rides/$id/review',
        body: {
          'rating': rating,
          'tags': ?tags,
          if (comment != null && comment.isNotEmpty) 'comment': comment,
        },
      );

  // -------------------------------------------------------------------- wallet

  Future<WalletOverview> wallet() async => WalletOverview.fromJson(await _get('/api/v1/wallet'));

  Future<List<WalletTransaction>> walletTransactions({int limit = 30, DateTime? before}) async {
    final rows = await _getList(
      '/api/v1/wallet/transactions',
      'items',
      query: {'limit': limit, if (before != null) 'before': before.toUtc().toIso8601String()},
    );
    return rows.map(WalletTransaction.fromJson).toList();
  }

  Future<PaymentIntent> topUp(int amountMinor, {String? idempotencyKey}) async =>
      PaymentIntent.fromJson(
        await _post(
          '/api/v1/wallet/topup',
          body: {'amount_minor': amountMinor},
          idempotencyKey: idempotencyKey ?? newIdempotencyKey(),
        ),
      );

  Future<PaymentIntent> paymentIntent(String id) async =>
      PaymentIntent.fromJson(await _get('/api/v1/payments/intents/$id'));

  // -------------------------------------------------------------------- driver

  Future<DriverProfile> driverProfile() async =>
      DriverProfile.fromJson(await _get('/api/v1/driver/profile'));

  Future<DriverProfile> applyAsDriver({
    required String make,
    required String model,
    required String plate,
    required String color,
    required List<String> categories,
  }) async => DriverProfile.fromJson(
    await _post(
      '/api/v1/driver/profile',
      body: {
        'vehicle_make': make,
        'vehicle_model': model,
        'vehicle_plate': plate,
        'vehicle_color': color,
        'categories': categories,
      },
    ),
  );

  Future<void> goOnline(List<String> categories) =>
      _post('/api/v1/driver/online', body: {'categories': categories});

  Future<void> goOffline() => _post('/api/v1/driver/offline');

  Future<void> pushLocation({
    required double lat,
    required double lng,
    double? heading,
    double? speedMps,
  }) => _post(
    '/api/v1/driver/location',
    body: {
      'lat': lat,
      'lng': lng,
      'heading': ?heading,
      'speed_mps': ?speedMps,
      'ts': DateTime.now().millisecondsSinceEpoch,
    },
  );

  Future<List<RideOffer>> driverOffers() async {
    final rows = await _getList('/api/v1/driver/offers', 'items');
    return rows.map(RideOffer.fromJson).toList();
  }

  Future<Ride> acceptOffer(String offerId) async => Ride.fromJson(
    await _post('/api/v1/driver/offers/$offerId/accept', idempotencyKey: newIdempotencyKey()),
  );

  Future<void> declineOffer(String offerId) => _post('/api/v1/driver/offers/$offerId/decline');

  Future<Ride?> driverActiveRide() async {
    final j = await _get('/api/v1/driver/rides/active');
    if (j.isEmpty || j['id'] == null) return null;
    return Ride.fromJson(j);
  }

  Future<Ride> driverArrived(String rideId) async =>
      Ride.fromJson(await _post('/api/v1/driver/rides/$rideId/arrived'));

  /// Nisja kërkon vërtetimin e marrjes: ose kodi 4-shifror, ose token-i i QR-së (§25).
  Future<Ride> driverStart(String rideId, {String? pickupCode, String? qrToken}) async =>
      Ride.fromJson(
        await _post(
          '/api/v1/driver/rides/$rideId/start',
          body: {'pickup_code': ?pickupCode, 'qr_token': ?qrToken},
        ),
      );

  Future<Ride> driverComplete(String rideId) async => Ride.fromJson(
    await _post('/api/v1/driver/rides/$rideId/complete', idempotencyKey: newIdempotencyKey()),
  );

  Future<Ride> driverCancel(String rideId, {String? reason}) async =>
      Ride.fromJson(await _post('/api/v1/driver/rides/$rideId/cancel', body: {'reason': ?reason}));

  Future<Earnings> earnings() async => Earnings.fromJson(await _get('/api/v1/driver/earnings'));

  Future<BankAccount?> bankAccount() async {
    final j = await _get('/api/v1/driver/bank-account');
    if (j.isEmpty || j['iban_masked'] == null) return null;
    return BankAccount.fromJson(j);
  }

  Future<BankAccount> saveBankAccount({required String holderName, required String iban}) async =>
      BankAccount.fromJson(
        await _post('/api/v1/driver/bank-account', body: {'holder_name': holderName, 'iban': iban}),
      );

  Future<DocumentsOverview> driverDocuments() async =>
      DocumentsOverview.fromJson(await _get('/api/v1/driver/documents'));

  /// Kthen `{upload_url, document_id}`; skedari ngarkohet drejt në S3 me PUT.
  Future<Map<String, dynamic>> documentUploadUrl({
    required String type,
    required String contentType,
    required int sizeBytes,
  }) => _post(
    '/api/v1/driver/documents/upload-url',
    body: {'type': type, 'content_type': contentType, 'size_bytes': sizeBytes},
  );

  // ------------------------------------------------------------------- support

  Future<Map<String, dynamic>> createTicket({
    required String category,
    required String subject,
    required String body,
    String? rideId,
  }) => _post(
    '/api/v1/support/tickets',
    body: {'category': category, 'subject': subject, 'body': body, 'ride_id': ?rideId},
  );

  Future<void> reportSafety({required String kind, String? rideId, String? note}) =>
      _post('/api/v1/safety/reports', body: {'kind': kind, 'ride_id': ?rideId, 'note': ?note});
}
