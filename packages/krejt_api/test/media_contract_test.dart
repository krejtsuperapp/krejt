import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';

/// Përgjigjet sipas shtegut; regjistron çdo kërkesë (shteg, metodë, trup) për verifikim.
class _Script implements HttpClientAdapter {
  _Script(this.replies);
  final Map<String, String> replies;
  final seen = <RequestOptions>[];

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? _, Future<void>? _) async {
    seen.add(options);
    final key = '${options.method} ${options.path}';
    final body = replies[key] ?? '{}';
    return ResponseBody.fromString(
      body,
      options.method == 'DELETE' ? 204 : 200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

KrejtApi _api(_Script script) => KrejtApi(
  config: const ApiConfig(
    baseUrl: 'http://localhost',
    appId: 'customer',
    platform: 'android',
    appVersion: '1.0.0',
  ),
  session: Session(store: MemoryStore()),
  dio: Dio()..httpClientAdapter = script,
  uploader: Dio()..httpClientAdapter = script,
);

Map<String, dynamic> _body(RequestOptions o) =>
    jsonDecode(jsonEncode(o.data)) as Map<String, dynamic>;

// Ngarkimi i imazhit publik ka tre hapa; kontrata e serverit është: kind + target_id +
// content_type + size_bytes → object_key + upload; PUT te URL-ja e nënshkruar; pastaj
// konfirmimi me object_key kthen url-në publike.
void main() {
  test('ngarkimi i logos: kërkon URL, PUT te bucket-i, konfirmon me çelësin', () async {
    final script = _Script({
      'POST /api/v1/media/upload-url': '{"object_key":"media/merchant_logo/m1/x.png","upload":{"url":"http://localhost/put-here","method":"PUT","headers":{"Content-Type":"image/png"},"expires_at":"2026-09-04T10:00:00Z"}}',
      'POST /api/v1/media': '{"kind":"merchant_logo","target_id":"m1","object_key":"media/merchant_logo/m1/x.png","url":"https://media.example/media/merchant_logo/m1/x.png"}',
      'PUT /put-here': '',
    });
    final url = await _api(script).uploadMedia(
      kind: 'merchant_logo',
      targetId: 'm1',
      bytes: [1, 2, 3],
      contentType: 'image/png',
    );
    expect(url, 'https://media.example/media/merchant_logo/m1/x.png');

    final ask = script.seen.firstWhere((o) => o.path == '/api/v1/media/upload-url');
    expect(_body(ask), {
      'kind': 'merchant_logo',
      'target_id': 'm1',
      'content_type': 'image/png',
      'size_bytes': 3,
    });
    final put = script.seen.firstWhere((o) => o.method == 'PUT');
    expect(put.uri.toString(), 'http://localhost/put-here');
    expect(put.headers['Content-Type'], 'image/png');
    // Bucket-i s'ka pse ta shohë token-in e sesionit.
    expect(put.headers.containsKey('Authorization'), isFalse);
    final confirm = script.seen.firstWhere((o) => o.path == '/api/v1/media');
    expect(_body(confirm), {'object_key': 'media/merchant_logo/m1/x.png'});
  });

  test('fotoja e profilit nuk dërgon target_id', () async {
    final script = _Script({
      'POST /api/v1/media/upload-url': '{"object_key":"media/user_photo/u1/x.jpg","upload":{"url":"http://localhost/put","method":"PUT","headers":{}}}',
      'POST /api/v1/media': '{"url":null}',
    });
    final url = await _api(script)
        .uploadMedia(kind: 'user_photo', bytes: [1], contentType: 'image/jpeg');
    expect(url, isNull);
    final ask = script.seen.firstWhere((o) => o.path == '/api/v1/media/upload-url');
    expect(_body(ask).containsKey('target_id'), isFalse);
  });

  test('heqja: DELETE /media/{kind} me target_id si query', () async {
    final script = _Script({});
    await _api(script).removeMedia(kind: 'product_image', targetId: 'p9');
    final del = script.seen.single;
    expect(del.method, 'DELETE');
    expect(del.uri.path, '/api/v1/media/product_image');
    expect(del.uri.queryParameters['target_id'], 'p9');
  });

  test('modelet lexojnë URL-të publike', () {
    final m = Merchant.fromJson({
      'id': 'm1',
      'name': 'Prova',
      'pickup': null,
      'logo_url': 'https://media.example/l.png',
      'cover_url': null,
    });
    expect(m.logoUrl, 'https://media.example/l.png');
    expect(m.coverUrl, isNull);
    final p = Product.fromJson({
      'id': 'p1',
      'name': 'Qebapa',
      'image_url': 'https://media.example/p.jpg',
    });
    expect(p.imageUrl, 'https://media.example/p.jpg');
    final me = Me.fromJson({'id': 'u1', 'photo_url': 'https://media.example/u.jpg'});
    expect(me.photoUrl, 'https://media.example/u.jpg');
  });
}
