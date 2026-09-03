/// Tipat që ekranet i njohin. Asnjë ofrues nuk shfaqet këtu, ndaj ndërrimi i tij nuk i prek ata.
library;

import 'dart:math' as math;

/// Një pikë gjeografike. E veçantë nga `LatLng` i API-së me qëllim: harta nuk duhet ta njohë
/// modelin e serverit, dhe ekranet e bëjnë kthimin me një rresht.
class MapPoint {
  const MapPoint(this.lat, this.lng);

  final double lat;
  final double lng;

  @override
  bool operator ==(Object other) => other is MapPoint && other.lat == lat && other.lng == lng;

  @override
  int get hashCode => Object.hash(lat, lng);

  @override
  String toString() => 'MapPoint($lat, $lng)';
}

/// Roli i shenjës. Ngjyrën dhe ikonën i vendos harta, që të mos ndryshojnë nga ekrani në ekran.
enum MapMarkerKind { pickup, dropoff, driver, place }

class MapMarker {
  const MapMarker({required this.point, required this.kind, this.label});

  final MapPoint point;
  final MapMarkerKind kind;

  /// Tekst i shkurtër për lexuesit e ekranit; harta nuk e vizaton.
  final String? label;
}

/// Kutia që i mbyll të gjitha pikat, me pak hapësirë anash që shenjat të mos preken me buzët.
class MapBounds {
  const MapBounds({
    required this.south,
    required this.west,
    required this.north,
    required this.east,
  });

  final double south;
  final double west;
  final double north;
  final double east;

  static MapBounds around(Iterable<MapPoint> points) {
    final list = points.toList(growable: false);
    if (list.isEmpty) {
      // Prishtina, që harta të mos jetë kurrë bosh ndërsa pritet pozicioni i parë.
      return const MapBounds(south: 42.6429, west: 21.1355, north: 42.6829, east: 21.1955);
    }
    var south = list.first.lat, north = list.first.lat;
    var west = list.first.lng, east = list.first.lng;
    for (final p in list.skip(1)) {
      south = math.min(south, p.lat);
      north = math.max(north, p.lat);
      west = math.min(west, p.lng);
      east = math.max(east, p.lng);
    }
    // Një pikë e vetme nuk ka shtrirje; pa këtë, harta do të kërkonte zmadhim të pafund.
    const minSpan = 0.004; // ~450 m
    if (north - south < minSpan) {
      final mid = (north + south) / 2;
      south = mid - minSpan / 2;
      north = mid + minSpan / 2;
    }
    if (east - west < minSpan) {
      final mid = (east + west) / 2;
      west = mid - minSpan / 2;
      east = mid + minSpan / 2;
    }
    return MapBounds(south: south, west: west, north: north, east: east);
  }

  MapPoint get center => MapPoint((south + north) / 2, (west + east) / 2);
}
