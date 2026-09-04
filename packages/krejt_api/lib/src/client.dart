import 'dart:async';
import 'dart:math';

import 'package:dio/dio.dart';

import 'errors.dart';
import 'models/config.dart';
import 'models/driver.dart';
import 'models/order.dart';
import 'models/parcel.dart';
import 'models/places.dart';
import 'models/promo.dart';
import 'models/legal.dart';
import 'models/service.dart';
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
  KrejtApi({
    required this.config,
    required this.session,
    Dio? dio,
    Dio? uploader,
    this.locale = 'sq',
  }) : _dio = dio ?? Dio(),
       // Ngarkimet e nënshkruara shkojnë te bucket-i me një klient pa interceptorët tanë:
       // token-i i sesionit nuk i dërgohet kurrë një hosti tjetër.
       _uploader = uploader ?? Dio() {
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
  final Dio _uploader;

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

  /// Kushtet (`terms`) ose Politika e privatësisë (`privacy`). Pa kyçje me qëllim: ekrani i hyrjes
  /// i lidh para se përdoruesi të ketë sesion.
  Future<LegalDocument> legalDocument(String doc, {required String lang}) async =>
      LegalDocument.fromJson(await _get('/api/v1/legal/$doc', query: {'lang': lang}, anon: true));

  // --------------------------------------------------------------------- auth

  /// Kërkon kodin njëpërdorimësh. Serveri kthen të njëjtën përgjigje edhe kur numri nuk ekziston.
  Future<void> requestOtp(String phone) =>
      _post('/api/v1/auth/otp/request', body: {'phone': phone}, anon: true);

  Future<Me> verifyOtp({required String phone, required String code, String? deviceName}) async {
    final j = await _post(
      '/api/v1/auth/otp/verify',
      body: {
        'phone': phone,
        'code': code,
        'locale': locale,
        // Serveri e kërkon pajisjen si objekt me id dhe platformë; pa to kthen 422.
        'device': {
          'id': session.deviceId,
          'name': deviceName ?? config.appId,
          'platform': config.platform,
        },
      },
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

  /// Regjistron token-in e pajisjes; serveri e lidh me llogarinë dhe e heq kur dërgesa dështon.
  Future<void> registerPushToken({
    required String token,
    required String platform,
    String? locale,
  }) => _post(
    '/api/v1/notifications/push-token',
    body: {'token': token, 'platform': platform, 'locale': ?locale},
  );

  /// Heq token-in në dalje, që njoftimet e llogarisë të mos vijnë më te kjo pajisje.
  Future<void> removePushToken(String token) async {
    await _send(
      () => _dio.delete<dynamic>('/api/v1/notifications/push-token', data: {'token': token}),
    );
  }

  // ------------------------------------------------------------------ realtime

  Future<Map<String, dynamic>> realtimeToken() => _post('/api/v1/realtime/token');

  /// Token-i i abonimit për një kanal; serveri vendos nëse ky përdorues e sheh.
  Future<Map<String, dynamic>> realtimeSubscribe(String channel) =>
      _post('/api/v1/realtime/subscribe', body: {'channel': channel});

  // --------------------------------------------------------------------- rides

  // --------------------------------------------------------------- shërbimet

  Future<List<ServiceCategory>> serviceCategories() async {
    final rows = await _getList('/api/v1/services/categories', 'items');
    return rows.map(ServiceCategory.fromJson).toList();
  }

  Future<ServiceRequest> createServiceRequest({
    required String categoryId,
    required String title,
    required String description,
    required String addressLine1,
    required LatLng address,
    required String paymentMethod,
    String? addressInstructions,
    DateTime? preferredAt,
    List<String> photoKeys = const [],
    String? idempotencyKey,
  }) async => ServiceRequest.fromJson(
    await _post(
      '/api/v1/services/requests',
      idempotencyKey: idempotencyKey ?? newIdempotencyKey(),
      body: {
        'category_id': categoryId,
        'title': title,
        'description': description,
        'address_line1': addressLine1,
        'address': address.toJson(),
        'payment_method': paymentMethod,
        'address_instructions': ?addressInstructions,
        'preferred_at': ?preferredAt?.toUtc().toIso8601String(),
        if (photoKeys.isNotEmpty) 'photo_keys': photoKeys,
      },
    ),
  );

  Future<ServiceRequest> serviceRequest(String id) async =>
      ServiceRequest.fromJson(await _get('/api/v1/services/requests/$id'));

  Future<List<ServiceRequest>> serviceRequests({int limit = 20}) async {
    final rows = await _getList('/api/v1/services/requests', 'items', query: {'limit': limit});
    return rows.map(ServiceRequest.fromJson).toList();
  }

  /// Klienti zgjedh një ofertë; çmimi i saj bëhet çmimi i punës.
  Future<ServiceRequest> acceptServiceOffer(String requestId, String offerId) async =>
      ServiceRequest.fromJson(
        await _post('/api/v1/services/requests/$requestId/accept', body: {'offer_id': offerId}),
      );

  Future<ServiceRequest> cancelServiceRequest(String id, {String? reason}) async =>
      ServiceRequest.fromJson(
        await _post('/api/v1/services/requests/$id/cancel', body: {'reason': ?reason}),
      );

  // --------------------------------------------------------------- mjeshtri

  Future<ServiceProviderProfile> serviceProviderProfile() async =>
      ServiceProviderProfile.fromJson(await _get('/api/v1/services/provider'));

  Future<ServiceProviderProfile> applyAsServiceProvider({
    required List<String> categories,
    required String city,
    String? businessName,
    String? bio,
    String? phonePublic,
  }) async => ServiceProviderProfile.fromJson(
    await _post(
      '/api/v1/services/provider',
      body: {
        'categories': categories,
        'city': city,
        'business_name': ?businessName,
        'bio': ?bio,
        'phone_public': ?phonePublic,
      },
    ),
  );

  Future<List<ServiceOpenRequest>> openServiceRequests({int limit = 20}) async {
    final rows = await _getList('/api/v1/services/provider/open', 'items', query: {'limit': limit});
    return rows.map(ServiceOpenRequest.fromJson).toList();
  }

  Future<List<ServiceRequest>> myServiceJobs({int limit = 20}) async {
    final rows = await _getList('/api/v1/services/provider/jobs', 'items', query: {'limit': limit});
    return rows.map(ServiceRequest.fromJson).toList();
  }

  /// Çmimin e vendos mjeshtri; ndryshimi lejohet derisa klienti të zgjedhë.
  Future<ServiceOffer> makeServiceOffer(
    String requestId, {
    required int priceMinor,
    String? note,
    DateTime? canStartAt,
  }) async => ServiceOffer.fromJson(
    await _post(
      '/api/v1/services/provider/requests/$requestId/offer',
      body: {
        'price_minor': priceMinor,
        'note': ?note,
        'can_start_at': ?canStartAt?.toUtc().toIso8601String(),
      },
    ),
  );

  Future<void> withdrawServiceOffer(String offerId) async {
    await _post('/api/v1/services/provider/offers/$offerId/withdraw');
  }

  Future<ServiceRequest> startServiceJob(String id) async =>
      ServiceRequest.fromJson(await _post('/api/v1/services/provider/requests/$id/start'));

  Future<ServiceRequest> completeServiceJob(String id) async =>
      ServiceRequest.fromJson(await _post('/api/v1/services/provider/requests/$id/complete'));

  Future<ServiceRequest> releaseServiceJob(String id, {String? reason}) async =>
      ServiceRequest.fromJson(
        await _post('/api/v1/services/provider/requests/$id/release', body: {'reason': ?reason}),
      );

  // ------------------------------------------------------------------ kupona

  /// Kontrollo një kupon para checkout-it; zbritjen e llogarit serveri.
  Future<CouponApplied> checkCoupon({
    required String code,
    required String scope,
    required int amountMinor,
  }) async => CouponApplied.fromJson(
    await _post(
      '/api/v1/coupons/check',
      body: {'code': code, 'scope': scope, 'amount_minor': amountMinor},
    ),
  );

  // ----------------------------------------------------------------- parcels

  /// Çmimi i dërgesës së pakos; vlen dy minuta.
  Future<ParcelQuote> quoteParcel({
    required String size,
    required LatLng pickup,
    required LatLng dropoff,
    String? pickupAddress,
    String? dropoffAddress,
  }) async => ParcelQuote.fromJson(
    await _post(
      '/api/v1/parcels/quote',
      body: {
        'size': size,
        'pickup': pickup.toJson(),
        'dropoff': dropoff.toJson(),
        'pickup_address': ?pickupAddress,
        'dropoff_address': ?dropoffAddress,
      },
    ),
  );

  Future<Parcel> createParcel({
    required String quoteId,
    required String paymentMethod,
    required String recipientName,
    required String recipientPhone,
    String? pickupContactName,
    String? pickupContactPhone,
    String? note,
    String? couponCode,
    String? idempotencyKey,
  }) async => Parcel.fromJson(
    await _post(
      '/api/v1/parcels',
      idempotencyKey: idempotencyKey ?? newIdempotencyKey(),
      body: {
        'quote_id': quoteId,
        'payment_method': paymentMethod,
        'recipient_name': recipientName,
        'recipient_phone': recipientPhone,
        'pickup_contact_name': ?pickupContactName,
        'pickup_contact_phone': ?pickupContactPhone,
        'note': ?note,
        'coupon_code': ?couponCode,
      },
    ),
  );

  Future<Parcel> parcel(String id) async => Parcel.fromJson(await _get('/api/v1/parcels/$id'));

  /// Pakoja aktive e klientit; null kur nuk ka.
  Future<Parcel?> activeParcel() async => _parcelOrNull(await _get('/api/v1/parcels/active'));

  Future<List<Parcel>> parcelHistory({int limit = 20}) async {
    final rows = await _getList('/api/v1/parcels', 'items', query: {'limit': limit});
    return rows.map(Parcel.fromJson).toList();
  }

  Future<Parcel> cancelParcel(String id, {String? reason}) async =>
      Parcel.fromJson(await _post('/api/v1/parcels/$id/cancel', body: {'reason': ?reason}));

  // korrieri
  Future<List<ParcelOffer>> courierParcelOffers() async {
    final rows = await _getList('/api/v1/courier/parcel-offers', 'items');
    return rows.map(ParcelOffer.fromJson).toList();
  }

  Future<Parcel> acceptParcelOffer(String offerId) async =>
      Parcel.fromJson(await _post('/api/v1/courier/parcel-offers/$offerId/accept'));

  Future<void> declineParcelOffer(String offerId) async {
    await _post('/api/v1/courier/parcel-offers/$offerId/decline');
  }

  Future<Parcel?> courierActiveParcel() async =>
      _parcelOrNull(await _get('/api/v1/courier/parcels/active'));

  static Parcel? _parcelOrNull(Map<String, dynamic> j) {
    final p = _unwrap(j, 'parcel');
    return p == null ? null : Parcel.fromJson(p);
  }

  Future<Parcel> courierParcelPickup(String id, {required String code}) async =>
      Parcel.fromJson(await _post('/api/v1/courier/parcels/$id/pickup', body: {'code': code}));

  Future<Parcel> courierParcelDeliver(String id, {required String code}) async =>
      Parcel.fromJson(await _post('/api/v1/courier/parcels/$id/deliver', body: {'code': code}));

  Future<Parcel> courierParcelRelease(String id, {String? reason}) async => Parcel.fromJson(
    await _post('/api/v1/courier/parcels/$id/release', body: {'reason': ?reason}),
  );

  // ------------------------------------------------------------------ places

  /// Vende dhe adresa brenda Kosovës; afërsia i rendit më afër përdoruesit.
  Future<List<Place>> searchPlaces(String q, {LatLng? near, int limit = 8}) async {
    final rows = await _getList(
      '/api/v1/places/search',
      'items',
      query: {'q': q, 'limit': limit, 'lat': ?near?.lat, 'lng': ?near?.lng},
    );
    return rows.map(Place.fromJson).toList();
  }

  /// Adresa e një pike (pika e marrjes nga GPS-i); null kur ofruesi nuk njeh asgjë aty.
  Future<Place?> reversePlace(LatLng point) async {
    final m = await _get('/api/v1/places/reverse', query: {'lat': point.lat, 'lng': point.lng});
    final place = m['place'];
    return place is Map ? Place.fromJson(Map<String, dynamic>.from(place)) : null;
  }

  /// Gjeometria e rrugës për hartën; çmimin e jep vetëm quote-i.
  Future<RoutePath> routePath(LatLng from, LatLng to) async => RoutePath.fromJson(
    await _post('/api/v1/places/route', body: {'from': from.toJson(), 'to': to.toJson()}),
  );

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

  /// Mostra GPS. Serveri pret një grup (, deri në 50, të renditura sipas kohës);
  /// aplikacioni dërgon një të vetme për çdo interval, por kontrata mbetet ajo e grupit.
  Future<void> pushLocation({
    required double lat,
    required double lng,
    double? heading,
    double? speedMps,
  }) => _post(
    '/api/v1/driver/location',
    body: {
      'samples': [
        {
          'lat': lat,
          'lng': lng,
          'heading': ?heading,
          'speed_mps': ?speedMps,
          'ts': DateTime.now().millisecondsSinceEpoch,
        },
      ],
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

  /// Serveri e kthen udhëtimin të mbështjellë (`{"ride": …}`, `null` kur s'ka); pranohet edhe
  /// trupi i zhveshur. Një lexim i gabuar këtu fshinte udhëtimin sapo shoferi e pranonte.
  Future<Ride?> driverActiveRide() async {
    final j = _unwrap(await _get('/api/v1/driver/rides/active'), 'ride');
    return j == null ? null : Ride.fromJson(j);
  }

  Future<Ride> driverArrived(String rideId) async =>
      Ride.fromJson(await _post('/api/v1/driver/rides/$rideId/arrived'));

  /// Nisja kërkon vërtetimin e marrjes: ose kodi 4-shifror, ose token-i i QR-së (§25).
  Future<Ride> driverStart(
    String rideId, {
    String? pickupCode,
    String? qrToken,
  }) async => Ride.fromJson(
    await _post(
      '/api/v1/driver/rides/$rideId/start',
      // Serveri e quan `code` (OpenAPI); emri i vjetër `pickup_code` refuzohej si fushë e panjohur.
      body: {'code': ?pickupCode, 'qr_token': ?qrToken},
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

  /// Imazh publik (logo/kopertinë vendi, imazh produkti, foto profili): URL e nënshkruar →
  /// PUT drejt bucket-it → konfirmim që e lidh me pronarin. Kthen URL-në publike të re.
  /// `targetId`: vendi ose produkti; bosh për foton e profilit.
  Future<String?> uploadMedia({
    required String kind,
    String? targetId,
    required List<int> bytes,
    required String contentType,
  }) async {
    final signed = await _post(
      '/api/v1/media/upload-url',
      body: {
        'kind': kind,
        'target_id': ?targetId,
        'content_type': contentType,
        'size_bytes': bytes.length,
      },
    );
    final objectKey = signed['object_key']?.toString();
    final upload = signed['upload'];
    if (objectKey == null || upload is! Map) {
      throw ApiError(code: 'INTERNAL', messageKey: 'errors.internal', status: 0);
    }
    await _putSigned(upload, bytes, contentType);
    final confirmed = await _post('/api/v1/media', body: {'object_key': objectKey});
    return confirmed['url']?.toString();
  }

  /// Heq imazhin e pronarit (logo, kopertinë, imazh produkti, foto profili).
  Future<void> removeMedia({required String kind, String? targetId}) =>
      _delete('/api/v1/media/$kind${targetId == null ? '' : '?target_id=$targetId'}');

  /// PUT-i i nënshkruar shkon te bucket-i, jo te API-ja, me klientin e ngarkimit (pa
  /// interceptorët tanë).
  Future<void> _putSigned(Map<dynamic, dynamic> upload, List<int> bytes, String contentType) async {
    final headers = <String, String>{'Content-Type': contentType};
    final extra = upload['headers'];
    if (extra is Map) {
      extra.forEach((k, v) => headers[k.toString()] = v.toString());
    }
    try {
      await _uploader.putUri<dynamic>(
        Uri.parse(upload['url'].toString()),
        data: Stream<List<int>>.fromIterable([bytes]),
        options: Options(
          headers: {...headers, Headers.contentLengthHeader: bytes.length},
          validateStatus: (s) => s != null && s < 400,
        ),
      );
    } on DioException catch (e) {
      throw ApiError.fromDio(e);
    }
  }

  /// Ngarkimi i një dokumenti në tri hapa (§31): serveri nënshkruan URL-në, skedari shkon
  /// drejt në S3 pa kaluar nga API-ja, dhe pastaj serveri e konfirmon dhe e vë në radhë për shqyrtim.
  /// Bajtët nuk kalojnë kurrë nëpër log-e dhe URL-ja e nënshkruar skadon vetë.
  Future<DriverDocument> uploadDriverDocument({
    required String type,
    required List<int> bytes,
    required String contentType,
    DateTime? expiresOn,
  }) async {
    final signed = await _post(
      '/api/v1/driver/documents/upload-url',
      body: {'type': type, 'content_type': contentType, 'size_bytes': bytes.length},
    );
    final objectKey = signed['object_key']?.toString();
    final upload = signed['upload'];
    if (objectKey == null || upload is! Map) {
      throw ApiError(code: 'INTERNAL', messageKey: 'errors.internal', status: 0);
    }
    await _putSigned(upload, bytes, contentType);

    final confirmed = await _post(
      '/api/v1/driver/documents',
      body: {
        'type': type,
        'object_key': objectKey,
        if (expiresOn != null) 'expires_on': expiresOn.toIso8601String().substring(0, 10),
      },
    );
    return DriverDocument.fromJson(confirmed);
  }

  // ------------------------------------------------------- ushqimi dhe marketi

  /// Zbulimi publik: merchant-ët aktivë brenda 15 km, me distancën dhe nëse janë hapur (§21).
  Future<List<Merchant>> merchants({
    required double lat,
    required double lng,
    String? type,
    String? query,
    String? cuisine,
    int limit = 20,
  }) async {
    final rows = await _getList(
      '/api/v1/merchants',
      'items',
      query: {
        'lat': lat,
        'lng': lng,
        'type': ?type,
        if (query != null && query.isNotEmpty) 'q': query,
        'cuisine': ?cuisine,
        'limit': limit,
      },
    );
    return rows.map(Merchant.fromJson).toList();
  }

  Future<Merchant> merchantBySlug(String slug) async =>
      Merchant.fromJson(await _get('/api/v1/merchants/$slug', anon: true));

  Future<Menu> merchantMenu(String merchantId) async =>
      Menu.fromJson(await _get('/api/v1/merchants/$merchantId/menu', anon: true));

  /// Çmimi i shportës llogaritet nga serveri para se të krijohet ndonjë porosi (§19).
  Future<OrderQuote> quoteOrder({
    required String merchantId,
    required List<CartLine> lines,
    required String paymentMethod,
    required String fulfillment,
    String? addressId,
    String? note,
    String? couponCode,
  }) async => OrderQuote.fromJson(
    await _post(
      '/api/v1/orders/quote',
      body: _checkout(
        merchantId: merchantId,
        lines: lines,
        paymentMethod: paymentMethod,
        fulfillment: fulfillment,
        addressId: addressId,
        note: note,
        couponCode: couponCode,
      ),
    ),
  );

  Future<Order> createOrder({
    required String merchantId,
    required List<CartLine> lines,
    required String paymentMethod,
    required String fulfillment,
    String? addressId,
    String? note,
    String? couponCode,
    String? idempotencyKey,
  }) async => Order.fromJson(
    await _post(
      '/api/v1/orders',
      body: _checkout(
        merchantId: merchantId,
        lines: lines,
        paymentMethod: paymentMethod,
        fulfillment: fulfillment,
        addressId: addressId,
        note: note,
        couponCode: couponCode,
      ),
      idempotencyKey: idempotencyKey ?? newIdempotencyKey(),
    ),
  );

  Map<String, dynamic> _checkout({
    required String merchantId,
    required List<CartLine> lines,
    required String paymentMethod,
    required String fulfillment,
    String? addressId,
    String? note,
    String? couponCode,
  }) => {
    'merchant_id': merchantId,
    'items': lines.map((l) => l.toJson()).toList(),
    'payment_method': paymentMethod,
    'fulfillment': fulfillment,
    'address_id': ?addressId,
    if (note != null && note.isNotEmpty) 'note': note,
    if (couponCode != null && couponCode.isNotEmpty) 'coupon_code': couponCode,
  };

  Future<Order> order(String id) async => Order.fromJson(await _get('/api/v1/orders/$id'));

  Future<List<Order>> orderHistory({int limit = 20}) async {
    final rows = await _getList('/api/v1/orders', 'items', query: {'limit': limit});
    return rows.map(Order.fromJson).toList();
  }

  Future<Order> cancelOrder(String id, {String? reason}) async =>
      Order.fromJson(await _post('/api/v1/orders/$id/cancel', body: {'reason': ?reason}));

  // ------------------------------------------------------------------ korrieri

  Future<List<CourierOffer>> courierOffers() async {
    final rows = await _getList('/api/v1/courier/offers', 'items');
    return rows.map(CourierOffer.fromJson).toList();
  }

  Future<Order> acceptCourierOffer(String offerId) async => Order.fromJson(
    await _post('/api/v1/courier/offers/$offerId/accept', idempotencyKey: newIdempotencyKey()),
  );

  Future<void> declineCourierOffer(String offerId) =>
      _post('/api/v1/courier/offers/$offerId/decline');

  Future<Order?> courierActiveOrder() async {
    final j = _unwrap(await _get('/api/v1/courier/orders/active'), 'order');
    return j == null ? null : Order.fromJson(j);
  }

  /// `{"<key>": {...}}` → objekti; `{"<key>": null}` ose `{}` → null; objekti i zhveshur → vetë.
  static Map<String, dynamic>? _unwrap(Map<String, dynamic> j, String key) {
    if (j.containsKey(key)) {
      final inner = j[key];
      return inner is Map<String, dynamic> && inner['id'] != null ? inner : null;
    }
    return j['id'] != null ? j : null;
  }

  /// Marrja te merchant-i kërkon kodin 6-shkronjor të porosisë: pa të, korrieri s'e merr dot (§26).
  Future<Order> courierPickup(String orderId, {required String code}) async =>
      Order.fromJson(await _post('/api/v1/courier/orders/$orderId/pickup', body: {'code': code}));

  Future<Order> courierDeliver(String orderId) async => Order.fromJson(
    await _post('/api/v1/courier/orders/$orderId/deliver', idempotencyKey: newIdempotencyKey()),
  );

  Future<Order> courierRelease(String orderId, {String? reason}) async => Order.fromJson(
    await _post('/api/v1/courier/orders/$orderId/release', body: {'reason': ?reason}),
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
