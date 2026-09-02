import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';

void main() {
  group('paraja', () {
    test('formatohet ndryshe sipas gjuhës', () {
      expect(1240.money(), '12,40 €');
      expect(1240.money(locale: 'en'), '€12.40');
      expect(0.money(), '0,00 €');
      expect(5.money(), '0,05 €');
    });

    test('shuma negative mban minusin para simbolit', () {
      expect((-750).money(), '-7,50 €');
      expect((-750).money(locale: 'en'), '-€7.50');
    });

    test('distanca kalon në kilometra vetëm mbi 950 m', () {
      expect(formatDistance(940), '940 m');
      expect(formatDistance(1500), '1,5 km');
      expect(formatDistance(1500, locale: 'en'), '1.5 km');
      expect(formatDistance(24300), '24 km');
    });

    test('kohëzgjatja lexohet si njeri', () {
      expect(formatDuration(30), '< 1 min');
      expect(formatDuration(420), '7 min');
      expect(formatDuration(3600), '1 h');
      expect(formatDuration(5400), '1 h 30 min');
    });
  });

  group('gabimet', () {
    test('zarfi i serverit lexohet i plotë', () {
      final res = Response<dynamic>(
        requestOptions: RequestOptions(path: '/x'),
        statusCode: 422,
        data: {
          'error': {
            'code': 'VALIDATION_FAILED',
            'message_key': 'errors.validation',
            'http_status': 422,
            'request_id': 'req_1',
            'retryable': false,
            'fields': {'phone': 'invalid'},
          },
        },
      );
      final e = ApiError.fromResponse(res);
      expect(e.code, 'VALIDATION_FAILED');
      expect(e.isValidation, isTrue);
      expect(e.fields['phone'], 'invalid');
      expect(e.requestId, 'req_1');
    });

    test('përgjigjja pa zarf bëhet gabim i brendshëm, jo tekst i papërpunuar', () {
      final res = Response<dynamic>(
        requestOptions: RequestOptions(path: '/x'),
        statusCode: 500,
        data: '<html>Internal Server Error</html>',
      );
      final e = ApiError.fromResponse(res);
      expect(e.code, 'INTERNAL');
      expect(e.messageKey, 'errors.internal');
    });

    test('humbja e lidhjes njihet si offline dhe mund të riprovohet', () {
      final e = ApiError.fromDio(
        DioException(
          requestOptions: RequestOptions(path: '/x'),
          type: DioExceptionType.connectionError,
        ),
      );
      expect(e.isOffline, isTrue);
      expect(e.retryable, isTrue);
    });
  });

  group('modelet', () {
    test('udhëtimi lexon gjendjen me nënvizim dhe çmimin final', () {
      final r = Ride.fromJson({
        'id': 'r1',
        'category': 'comfort',
        'state': 'in_progress',
        'payment_method': 'wallet',
        'payment_status': 'pending',
        'pickup': {'lat': 42.66, 'lng': 21.16},
        'dropoff': {'lat': 42.67, 'lng': 21.17},
        'price_quoted_minor': 350,
        'price_final_minor': 380,
        'currency': 'EUR',
        'requested_at': '2026-09-02T10:00:00Z',
        'driver_id': 'd1',
      });
      expect(r.state, RideState.inProgress);
      expect(r.category, RideCategory.comfort);
      expect(r.isActive, isTrue);
      expect(r.priceMinor, 380);
    });

    test('udhëtimi pa çmim final tregon atë të ofertës', () {
      final r = Ride.fromJson({
        'id': 'r2',
        'state': 'matching',
        'pickup': {'lat': 0, 'lng': 0},
        'dropoff': {'lat': 0, 'lng': 0},
        'price_quoted_minor': 250,
        'requested_at': '2026-09-02T10:00:00Z',
      });
      expect(r.priceMinor, 250);
      expect(r.chatOpen, isFalse);
    });

    test('gjendja e panjohur nuk rrëzon aplikacionin', () {
      expect(rideStateFrom('diçka_e_re'), RideState.noDriver);
      expect(rideCategoryFrom('helikopter'), RideCategory.economy);
    });

    test('konfigurimi me update_state=required bllokon hyrjen', () {
      final c = PublicConfig.fromJson({
        'update_state': 'required',
        'flags': {'food': true, 'market': false},
        'server_time': '2026-09-02T10:00:00Z',
        'config_ttl_s': 120,
      });
      expect(c.updateState, UpdateState.updateRequired);
      expect(c.blocksApp, isTrue);
      expect(c.flag('food'), isTrue);
      expect(c.flag('market'), isFalse);
      expect(c.flag('nuk_ekziston', fallback: true), isTrue);
    });

    test('oferta e shoferit njeh skadimin', () {
      final o = RideOffer.fromJson({
        'id': 'o1',
        'ride_id': 'r1',
        'expires_at': DateTime.now().subtract(const Duration(seconds: 1)).toIso8601String(),
        'pickup': {'lat': 0, 'lng': 0},
        'dropoff': {'lat': 0, 'lng': 0},
        'earnings_minor': 300,
      });
      expect(o.expired, isTrue);
      expect(o.secondsLeft, 0);
    });

    test('emri i shfaqur bie te telefoni i maskuar', () {
      final me = Me.fromJson({
        'id': 'u1',
        'phone': '+38344123456',
        'locale': 'sq',
        'capabilities': ['RIDE_DRIVER'],
        'wallet': {'balance_minor': 1500, 'currency': 'EUR'},
      });
      expect(me.displayName, '•••456');
      expect(me.isDriver, isTrue);
      expect(me.wallet.balanceMinor, 1500);
    });
  });

  test('çelësat e idempotencës janë unikë dhe në formatin UUID', () {
    final a = newIdempotencyKey();
    final b = newIdempotencyKey();
    expect(a, isNot(b));
    expect(
      RegExp(r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$').hasMatch(a),
      isTrue,
    );
  });
}
