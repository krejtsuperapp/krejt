import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';

void main() {
  group('websocketUrlFor', () {
    test('në prodhim Centrifugo rri pas të njëjtit domen, me wss', () {
      expect(
        RealtimeClient.websocketUrlFor('https://dev.krejt.app'),
        'wss://dev.krejt.app/connection/websocket',
      );
    });

    test('lokalisht API-ja është në 8080 dhe Centrifugo në 8000', () {
      expect(
        RealtimeClient.websocketUrlFor('http://10.0.2.2:8080'),
        'ws://10.0.2.2:8000/connection/websocket',
      );
    });

    test('një adresë e sigurt me portë e ruan portën', () {
      expect(
        RealtimeClient.websocketUrlFor('https://staging.krejt.app:8443'),
        'wss://staging.krejt.app:8443/connection/websocket',
      );
    });
  });

  group('RealtimeEvent', () {
    test('ngjarja e domenit e mban ngarkesën te data', () {
      const e = RealtimeEvent('ride:1', {
        'type': 'RideAssigned',
        'data': {'ride_id': '1', 'driver_id': 'd'},
      });
      expect(e.type, 'RideAssigned');
      expect(e.payload['driver_id'], 'd');
    });

    test('pozicioni i shoferit vjen i sheshtë', () {
      const e = RealtimeEvent('ride:1', {'type': 'driver_location', 'lat': 42.6, 'lng': 21.1});
      expect(e.type, 'driver_location');
      expect(e.payload['lat'], 42.6);
    });
  });
}
