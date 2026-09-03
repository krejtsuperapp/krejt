import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';

/// Kthen një trup fiks për çdo kërkesë; kap shtegun që u kërkua.
class _Reply implements HttpClientAdapter {
  _Reply(this.body);
  final String body;
  String? path;

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? _, Future<void>? _) async {
    path = options.path;
    return ResponseBody.fromString(
      body,
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

KrejtApi _api(_Reply reply) => KrejtApi(
  config: const ApiConfig(
    baseUrl: 'http://localhost',
    appId: 'driver',
    platform: 'android',
    appVersion: '1.0.0',
  ),
  session: Session(store: MemoryStore()),
  dio: Dio()..httpClientAdapter = reply,
);

const _ride =
    '{"id":"r1","state":"assigned","pickup":{"lat":42.66,"lng":21.16},'
    '"dropoff":{"lat":42.67,"lng":21.17},"price_quoted_minor":300,"currency":"EUR",'
    '"requested_at":"2026-09-03T20:31:57Z","driver_id":"d1"}';

const _order =
    '{"id":"o1","code":"K7F3QA","state":"courier_assigned","courier_id":"d1",'
    '"total_minor":1050,"created_at":"2026-09-03T20:31:57Z","items":[]}';

// Serveri i kthen udhëtimin dhe porosinë aktive të mbështjella: {"ride": …} / {"order": …},
// me null kur s'ka. Leximi i gabuar i mbështjellësit e bënte aplikacionin e shoferit ta "humbte"
// udhëtimin 15 s pasi e pranonte — sondazhi i radhës lexonte "asgjë" dhe mbyllte ekranin.
void main() {
  group('udhëtimi aktiv i shoferit', () {
    test('lexon udhëtimin nga mbështjellësi "ride"', () async {
      final reply = _Reply('{"ride":$_ride}');
      final ride = await _api(reply).driverActiveRide();
      expect(reply.path, '/api/v1/driver/rides/active');
      expect(ride?.id, 'r1');
      expect(ride?.state, RideState.assigned);
    });

    test('"ride": null do të thotë s\'ka udhëtim', () async {
      expect(await _api(_Reply('{"ride":null}')).driverActiveRide(), isNull);
    });

    test('trupi bosh do të thotë s\'ka udhëtim', () async {
      expect(await _api(_Reply('{}')).driverActiveRide(), isNull);
    });

    test('pranon edhe trupin e zhveshur', () async {
      final ride = await _api(_Reply(_ride)).driverActiveRide();
      expect(ride?.id, 'r1');
    });
  });

  group('porosia aktive e korrierit', () {
    test('lexon porosinë nga mbështjellësi "order"', () async {
      final reply = _Reply('{"order":$_order}');
      final order = await _api(reply).courierActiveOrder();
      expect(reply.path, '/api/v1/courier/orders/active');
      expect(order?.id, 'o1');
      expect(order?.courierId, 'd1');
    });

    test('"order": null do të thotë s\'ka porosi', () async {
      expect(await _api(_Reply('{"order":null}')).courierActiveOrder(), isNull);
    });
  });
}
