/// Kanali i gjallë (§42): Centrifugo pas ALB-së, me token-a lidhjeje dhe abonimi që i lëshon
/// serveri ynë. Ekranet marrin ngjarje si "shtytje" — kur vjen një, rifreskojnë gjendjen nga
/// API-ja ose e zbatojnë drejtpërdrejt (pozicioni i shoferit). Pa lidhje, ekrani mbetet i
/// përdorshëm: pyetja periodike mbetet si rrugë rezervë, vetëm më e rrallë.
library;

import 'dart:async';
import 'dart:convert';

import 'package:centrifuge/centrifuge.dart' as cf;

import 'client.dart';

/// Një ngjarje nga kanali, e dekoduar nga JSON-i që publikon serveri.
class RealtimeEvent {
  const RealtimeEvent(this.channel, this.data);

  final String channel;
  final Map<String, dynamic> data;

  /// `type` i ngjarjes: `RideAssigned`, `driver_location`, …
  String get type => (data['type'] ?? '').toString();

  /// Ngjarjet e domenit e mbajnë ngarkesën te `data`; pozicioni i shoferit vjen i sheshtë.
  Map<String, dynamic> get payload {
    final d = data['data'];
    return d is Map<String, dynamic> ? d : data;
  }
}

/// Lidhja e vetme e aplikacionit me Centrifugo-n. Krijohet një herë për sesion; abonimet
/// hapen dhe mbyllen sipas ekranit.
class RealtimeClient {
  RealtimeClient(this.api, {String? url}) : _url = url ?? websocketUrlFor(api.config.baseUrl);

  final KrejtApi api;
  final String _url;

  cf.Client? _client;
  final _subs = <String, cf.Subscription>{};

  /// Adresa e WebSocket-it nga ajo e API-së. Në prodhim Centrifugo rri pas të njëjtit domen
  /// (`/connection/*` te ALB-ja); lokalisht dëgjon në portën 8000 të docker-compose.
  static String websocketUrlFor(String apiBaseUrl) {
    final u = Uri.parse(apiBaseUrl);
    final secure = u.scheme == 'https';
    final local = u.port == 8080 && !secure;
    return Uri(
      scheme: secure ? 'wss' : 'ws',
      host: u.host,
      port: local ? 8000 : (u.hasPort ? u.port : null),
      path: '/connection/websocket',
    ).toString();
  }

  cf.Client _ensure() {
    return _client ??= cf.createClient(
      _url,
      cf.ClientConfig(
        // Token-i i lidhjes vjen nga serveri ynë dhe rifreskohet vetë kur Centrifugo e kërkon.
        getToken: (_) async => (await api.realtimeToken())['token'].toString(),
        minReconnectDelay: const Duration(seconds: 1),
        maxReconnectDelay: const Duration(seconds: 20),
      ),
    );
  }

  /// Abonohet te një kanal dhe kthen rrjedhën e ngjarjeve. Serveri vendos nëse ky përdorues
  /// e sheh kanalin; përgjigjja e tij është token-i i abonimit.
  Stream<RealtimeEvent> subscribe(String channel) {
    final client = _ensure();
    final sub = _subs[channel] ??= client.newSubscription(
      channel,
      cf.SubscriptionConfig(
        getToken: (_) async => (await api.realtimeSubscribe(channel))['token'].toString(),
      ),
    );
    final controller = StreamController<RealtimeEvent>();
    final listener = sub.publication.listen((p) {
      try {
        final decoded = jsonDecode(utf8.decode(p.data));
        if (decoded is Map<String, dynamic>) controller.add(RealtimeEvent(channel, decoded));
      } catch (_) {
        // Një mesazh i keqformuar nuk duhet ta prishë rrjedhën; rruga rezervë e mbulon.
      }
    });
    controller.onCancel = () async {
      await listener.cancel();
      if (!controller.hasListener) {
        _subs.remove(channel);
        await sub.unsubscribe();
      }
    };
    unawaited(sub.subscribe());
    unawaited(client.connect());
    return controller.stream;
  }

  Future<void> dispose() async {
    for (final s in _subs.values) {
      await s.unsubscribe();
    }
    _subs.clear();
    await _client?.disconnect();
    _client = null;
  }
}

/// Kanalet, në të njëjtën formë si te serveri (`realtime.RideChannel` etj.).
String rideChannel(String rideId) => 'ride:$rideId';
String orderChannel(String orderId) => 'order:$orderId';
String driverChannel(String driverId) => 'driver:$driverId';
String userChannel(String userId) => 'user:$userId';
