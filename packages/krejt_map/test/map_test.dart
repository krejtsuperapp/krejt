import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_map/krejt_map.dart';

void main() {
  group('MapConfig.fromEnv', () {
    test('pa çelës bie te skema, edhe kur ofruesi kërkohet shprehimisht', () {
      final cfg = MapConfig.fromEnv(provider: 'mapbox', token: '', style: '');
      expect(cfg.kind, MapProviderKind.schematic);
      expect(cfg.isLive, isFalse);
    });

    test('çelësi vetëm mjafton: ofruesi nënkuptohet', () {
      final cfg = MapConfig.fromEnv(provider: '', token: 'pk.test', style: '');
      expect(cfg.kind, MapProviderKind.mapbox);
      expect(cfg.mapboxStyle, 'mapbox/dark-v11');
    });

    test('një ofrues i panjohur nuk kalon si Mapbox', () {
      final cfg = MapConfig.fromEnv(provider: 'google', token: 'pk.test', style: '');
      expect(cfg.kind, MapProviderKind.schematic);
    });
  });

  group('MapBounds', () {
    test('një pikë e vetme merr shtrirje minimale, që zmadhimi të mos jetë i pafund', () {
      final b = MapBounds.around([const MapPoint(42.6629, 21.1655)]);
      expect(b.north - b.south, greaterThan(0));
      expect(b.east - b.west, greaterThan(0));
      expect(b.center.lat, closeTo(42.6629, 1e-9));
    });

    test('i mbyll të gjitha pikat', () {
      final b = MapBounds.around(const [MapPoint(42.6629, 21.1655), MapPoint(42.2139, 20.7397)]);
      expect(b.south, closeTo(42.2139, 1e-9));
      expect(b.north, closeTo(42.6629, 1e-9));
      expect(b.west, closeTo(20.7397, 1e-9));
      expect(b.east, closeTo(21.1655, 1e-9));
    });
  });

  group('MapboxStaticUrl', () {
    final url = const MapboxStaticUrl(token: 'pk.test', style: 'mapbox/dark-v11');

    test('shenjat vijnë si lng,lat — e kundërta e rendit që përdor kodi ynë', () {
      final built = url.build(
        markers: const [MapMarker(point: MapPoint(42.6629, 21.1655), kind: MapMarkerKind.pickup)],
        width: 400,
        height: 200,
      );
      expect(built, contains('pin-s-a+6A3DFF(21.165500,42.662900)'));
      expect(built, contains('/auto/'));
      expect(built, contains('padding=48'));
    });

    test('pa shenja përdoret një qendër, sepse `auto` nuk ka çfarë të kornizojë', () {
      final built = url.build(markers: const [], width: 400, height: 200);
      expect(built, isNot(contains('/auto/')));
      expect(built, contains('access_token=pk.test'));
    });

    test('brinja nuk e kalon kufirin e API-së', () {
      final built = url.build(
        markers: const [MapMarker(point: MapPoint(42.6, 21.1), kind: MapMarkerKind.driver)],
        width: 5000,
        height: 200,
      );
      expect(built, contains('/1280x200@2x'));
    });

    test('ekrani jo-retina nuk kërkon pamje të dyfishtë', () {
      final built = url.build(
        markers: const [MapMarker(point: MapPoint(42.6, 21.1), kind: MapMarkerKind.driver)],
        width: 300,
        height: 150,
        devicePixelRatio: 1,
      );
      expect(built, contains('/300x150?'));
    });

    test('çelësi nuk shkruhet te shtegu, vetëm te pyetja', () {
      final built = url.build(
        markers: const [MapMarker(point: MapPoint(42.6, 21.1), kind: MapMarkerKind.place)],
        width: 300,
        height: 150,
      );
      expect(built.split('?').first, isNot(contains('pk.test')));
    });
  });

  group('KMap.settle', () {
    const shown = [MapMarker(point: MapPoint(42.662900, 21.165500), kind: MapMarkerKind.driver)];

    test('lëvizja nën pragun e freskimit nuk e ndryshon pamjen', () {
      // Rreth dhjetë metra veri.
      const next = [MapMarker(point: MapPoint(42.662990, 21.165500), kind: MapMarkerKind.driver)];
      expect(KMap.settle(shown, next), same(shown));
    });

    test('as kur pikat bien mbi një kufi rrjete — prandaj matet distanca', () {
      // Pikërisht çifti që rrumbullakosja në rrjetë i ndante në dy qeliza.
      const a = [MapMarker(point: MapPoint(42.6629745, 21.165500), kind: MapMarkerKind.driver)];
      const b = [MapMarker(point: MapPoint(42.6629765, 21.165500), kind: MapMarkerKind.driver)];
      expect(KMap.settle(a, b), same(a));
    });

    test('lëvizja e dukshme e freskon', () {
      // Rreth njëqind metra veri.
      const next = [MapMarker(point: MapPoint(42.663800, 21.165500), kind: MapMarkerKind.driver)];
      expect(KMap.settle(shown, next), same(next));
    });

    test('një shenjë e re e freskon menjëherë, sa larg qoftë', () {
      const next = [
        MapMarker(point: MapPoint(42.662900, 21.165500), kind: MapMarkerKind.driver),
        MapMarker(point: MapPoint(42.662901, 21.165501), kind: MapMarkerKind.dropoff),
      ];
      expect(KMap.settle(shown, next), same(next));
    });

    test('distanca matet me ngushtimin e gjatësisë', () {
      // Një e mijta e gradës: në gjerësi ~111 m, në gjatësi më pak, sepse jemi në 42°.
      const p = MapPoint(42.0, 21.0);
      expect(KMap.metersBetween(p, const MapPoint(42.001, 21.0)), closeTo(111.3, 1));
      expect(KMap.metersBetween(p, const MapPoint(42.0, 21.001)), closeTo(82.7, 1));
    });
  });
}
