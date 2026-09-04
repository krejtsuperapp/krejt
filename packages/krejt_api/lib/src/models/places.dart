import 'ride.dart';

/// Një vend ose adresë e gjetur nga serveri (ofruesi i hartave mbetet pas tij).
class Place {
  const Place({required this.name, required this.address, required this.kind, required this.point});

  final String name;
  final String address;

  /// address | street | poi | place | locality | neighborhood
  final String kind;
  final LatLng point;

  /// Rreshti i dytë pa përsëritur emrin: "Rr. Agim Ramadani, Prishtinë" nën "Newborn".
  String get subtitle => address == name ? '' : address;

  factory Place.fromJson(Map<String, dynamic> j) => Place(
    name: (j['name'] ?? '').toString(),
    address: (j['address'] ?? '').toString(),
    kind: (j['kind'] ?? 'place').toString(),
    point: LatLng.fromJson(Map<String, dynamic>.from(j['point'] as Map)),
  );
}

/// Rruga me gjeometri mes dy pikave — vetëm për vizatim; çmimi vjen nga quote-i.
class RoutePath {
  const RoutePath({required this.distanceM, required this.durationS, required this.points});

  final int distanceM;
  final int durationS;
  final List<LatLng> points;

  factory RoutePath.fromJson(Map<String, dynamic> j) => RoutePath(
    distanceM: (j['distance_m'] as num?)?.toInt() ?? 0,
    durationS: (j['duration_s'] as num?)?.toInt() ?? 0,
    points: [
      for (final p in (j['path'] as List?) ?? const [])
        LatLng.fromJson(Map<String, dynamic>.from(p as Map)),
    ],
  );
}
