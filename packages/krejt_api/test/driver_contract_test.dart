import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';

/// Kap kërkesën pa e dërguar; kthen një përgjigje minimale që rrjedha të mos ndalet te rrjeti.
class _Capture implements HttpClientAdapter {
  final seen = <RequestOptions>[];

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? _, Future<void>? _) async {
    seen.add(options);
    return ResponseBody.fromString(
      '{}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

KrejtApi _api(_Capture capture) => KrejtApi(
  config: const ApiConfig(
    baseUrl: 'http://localhost',
    appId: 'driver',
    platform: 'android',
    appVersion: '1.0.0',
  ),
  session: Session(store: MemoryStore()),
  dio: Dio()..httpClientAdapter = capture,
);

Map<String, dynamic> _body(RequestOptions o) =>
    jsonDecode(jsonEncode(o.data)) as Map<String, dynamic>;

// Të dy kontratat u zbuluan nga prova e udhëtimit kundrejt serverit të vërtetë: aplikacioni
// i shoferit dërgonte formë të gabuar dhe serveri e refuzonte me 422 — pozicioni nuk mbërrinte
// kurrë dhe udhëtimi nuk nisej dot. Testet e mbajnë formën të lidhur me OpenAPI-n.
void main() {
  test('pozicioni dërgohet si grup mostrash (samples), siç e kërkon serveri', () async {
    final capture = _Capture();
    await _api(capture).pushLocation(lat: 42.6629, lng: 21.1655, heading: 90);

    final req = capture.seen.singleWhere((o) => o.path.endsWith('/driver/location'));
    final body = _body(req);
    expect(
      body.containsKey('lat'),
      isFalse,
      reason: 'forma e rrafshët refuzohet si fushë e panjohur',
    );
    final samples = body['samples'];
    expect(samples, isA<List>());
    final s = (samples as List).single as Map<String, dynamic>;
    expect(s['lat'], 42.6629);
    expect(s['lng'], 21.1655);
    expect(s['heading'], 90);
    expect(s['ts'], isA<int>(), reason: 'koha e pajisjes, ms Unix');
  });

  test('nisja e udhëtimit dërgon kodin si `code`, jo `pickup_code`', () async {
    final capture = _Capture();
    try {
      await _api(capture).driverStart('r1', pickupCode: '4821');
    } catch (_) {
      // Përgjigjja e rreme nuk është udhëtim; kërkesa tashmë u kap.
    }

    final req = capture.seen.singleWhere((o) => o.path.endsWith('/driver/rides/r1/start'));
    final body = _body(req);
    expect(body['code'], '4821');
    expect(body.containsKey('pickup_code'), isFalse);
    expect(body.containsKey('qr_token'), isFalse, reason: 'fushat bosh nuk dërgohen');
  });
}
