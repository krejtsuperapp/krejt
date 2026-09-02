import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';

/// Kap kërkesën pa dalë në rrjet, që forma e trupit të mund të kontrollohet.
class _Capture implements HttpClientAdapter {
  final List<RequestOptions> seen = [];
  final Map<String, Object?> reply;

  _Capture(this.reply);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    seen.add(options);
    return ResponseBody.fromString(
      jsonEncode(reply),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test('kyçja dërgon pajisjen si objekt me id dhe platformë', () async {
    // Serveri e refuzon me 422 nëse device.id ose device.platform mungojnë (§53).
    final capture = _Capture({
      'access_token': 'a',
      'refresh_token': 'r',
      'user_id': '11111111-1111-1111-1111-111111111111',
    });

    final dio = Dio()..httpClientAdapter = capture;
    final api = KrejtApi(
      config: const ApiConfig(
        baseUrl: 'http://localhost',
        appId: 'customer',
        platform: 'android',
        appVersion: '1.0.0',
      ),
      session: Session(store: MemoryStore()),
      dio: dio,
    );

    // Vetëm forma e kërkesës na intereson; përgjigjja e rreme e ndal rrjedhën më vonë.
    try {
      await api.verifyOtp(phone: '+38344123456', code: '123456');
    } catch (_) {
      // `me()` dështon me përgjigjen e rreme; kërkesa e parë tashmë u kap.
    }

    // Rrjedha bën dy kërkesa; na intereson e para, ajo e kyçjes.
    final verify = capture.seen.firstWhere((o) => o.path.endsWith('/auth/otp/verify'));
    final body = jsonDecode(jsonEncode(verify.data)) as Map<String, dynamic>;
    expect(body['phone'], '+38344123456');
    expect(body['code'], '123456');

    final device = body['device'];
    expect(device, isA<Map>(), reason: 'pajisja duhet objekt, jo fushë e rrafshët');
    expect((device as Map)['platform'], 'android');
    expect(device['id'], isNotNull, reason: 'serveri e refuzon kyçjen pa id pajisjeje');
    expect((device['id'] as String).isNotEmpty, isTrue);
  });
}
