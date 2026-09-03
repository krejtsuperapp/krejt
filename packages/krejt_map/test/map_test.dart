import 'package:flutter/material.dart';
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

  group('stili', () {
    test('emri i shkurtër bëhet adresë e SDK-së', () {
      const cfg = MapConfig(kind: MapProviderKind.mapbox, mapboxToken: 'pk.test');
      expect(cfg.mapboxStyleUri, 'mapbox://styles/mapbox/dark-v11');
    });

    test('një adresë e plotë kalon e paprekur', () {
      const cfg = MapConfig(
        kind: MapProviderKind.mapbox,
        mapboxToken: 'pk.test',
        mapboxStyle: 'mapbox://styles/krejt/abc123',
      );
      expect(cfg.mapboxStyleUri, 'mapbox://styles/krejt/abc123');
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

    test('lista bosh jep një kuti të vlefshme, që harta të mos jetë kurrë e pavendosur', () {
      final b = MapBounds.around(const []);
      expect(b.north, greaterThan(b.south));
      expect(b.east, greaterThan(b.west));
    });
  });

  group('KMap pa çelës', () {
    testWidgets('vizaton skemën, jo një kuti bosh', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: KMap(
              config: MapConfig(kind: MapProviderKind.schematic),
              markers: [
                MapMarker(point: MapPoint(42.6629, 21.1655), kind: MapMarkerKind.pickup),
                MapMarker(point: MapPoint(42.2139, 20.7397), kind: MapMarkerKind.dropoff),
              ],
              schematicCaption: 'Skemë — pa rrugë',
            ),
          ),
        ),
      );
      expect(find.byType(SchematicMap), findsOneWidget);
      expect(find.text('Skemë — pa rrugë'), findsOneWidget);
    });

    testWidgets('etiketa e ndihmës i kalon lexuesit të ekranit', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: KMap(
              config: MapConfig(kind: MapProviderKind.schematic),
              markers: [MapMarker(point: MapPoint(42.6629, 21.1655), kind: MapMarkerKind.place)],
              semanticsLabel: 'Harta e udhëtimit',
            ),
          ),
        ),
      );
      expect(find.bySemanticsLabel('Harta e udhëtimit'), findsOneWidget);
    });
  });
}
