/// Ndërtimi i adresës së pamjes te Mapbox-i. E ndarë nga widget-i që të provohet pa ekran.
library;

import 'model.dart';

class MapboxStaticUrl {
  const MapboxStaticUrl({required this.token, required this.style});

  final String token;
  final String style;

  /// Kufijtë e Static Images API-së: 1280 pikë për brinjë, dhe `@2x` e dyfishon rezultatin.
  static const _maxSide = 1280;

  /// Ngjyrat vijnë nga tokenët e markës, pa `#`, sepse adresa nuk e pranon.
  static const _colors = {
    MapMarkerKind.pickup: '6A3DFF',
    MapMarkerKind.dropoff: '19C37D',
    MapMarkerKind.driver: 'FFB020',
    MapMarkerKind.place: '855EFF',
  };

  /// Ikonat Maki që Mapbox-i njeh; emër i panjohur e kthen gjithë pamjen në gabim.
  static const _icons = {
    MapMarkerKind.pickup: 'a',
    MapMarkerKind.dropoff: 'b',
    MapMarkerKind.driver: 'car',
    MapMarkerKind.place: 'marker',
  };

  String build({
    required List<MapMarker> markers,
    required int width,
    required int height,
    double devicePixelRatio = 2,
  }) {
    final retina = devicePixelRatio >= 1.5;
    // Kërkohet madhësia logjike; `@2x` e dyfishon vetë, ndaj kërkesa nuk duhet dyfishuar këtu.
    final w = _clampSide(width);
    final h = _clampSide(height);

    final overlay = markers.isEmpty ? '' : markers.map(_pin).join(',');

    // Pa shenja nuk ka çfarë të kornizohet, ndaj përdoret qendra e paracaktuar në vend të `auto`.
    final view = markers.isEmpty
        ? () {
            final c = MapBounds.around(const []).center;
            return '${c.lng.toStringAsFixed(6)},${c.lat.toStringAsFixed(6)},12,0';
          }()
        : 'auto';

    final path = overlay.isEmpty ? view : '$overlay/$view';
    final query = Uri(
      queryParameters: {
        'access_token': token,
        // Hapësirë anash që shenjat të mos priten nga buza kur `auto` e shtrëngon kornizën.
        if (markers.isNotEmpty) 'padding': '48',
        'attribution': 'true',
        'logo': 'true',
      },
    ).query;

    return 'https://api.mapbox.com/styles/v1/$style/static/$path/${w}x$h${retina ? '@2x' : ''}?$query';
  }

  String _pin(MapMarker m) {
    final icon = _icons[m.kind] ?? 'marker';
    final color = _colors[m.kind] ?? '6A3DFF';
    final lng = m.point.lng.toStringAsFixed(6);
    final lat = m.point.lat.toStringAsFixed(6);
    return 'pin-s-$icon+$color($lng,$lat)';
  }

  static int _clampSide(int v) => v < 1
      ? 1
      : v > _maxSide
      ? _maxSide
      : v;
}
