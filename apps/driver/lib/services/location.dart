import 'package:geolocator/geolocator.dart';
import 'package:krejt_api/krejt_api.dart';

/// Përse pozicioni nuk u mor. Ekrani e përkthen në tekst dhe ofron rrugën tjetër
/// (zgjedhjen e një adrese të ruajtur), në vend që të mbetet bosh (§55).
enum LocationProblem { disabled, denied, deniedForever, failed }

class LocationResult {
  const LocationResult.ok(this.point) : problem = null;
  const LocationResult.error(this.problem) : point = null;

  final LatLng? point;
  final LocationProblem? problem;

  bool get isOk => point != null;
}

/// Pozicioni i pajisjes për pikën e marrjes. Kërkohet vetëm kur përdoruesi nis një udhëtim,
/// kurrë në sfond — aplikacioni i klientit nuk ndjek askënd (§27, §57).
class LocationService {
  const LocationService();

  Future<LocationResult> current() async {
    try {
      if (!await Geolocator.isLocationServiceEnabled()) {
        return const LocationResult.error(LocationProblem.disabled);
      }
      var permission = await Geolocator.checkPermission();
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
      }
      if (permission == LocationPermission.deniedForever) {
        return const LocationResult.error(LocationProblem.deniedForever);
      }
      if (permission == LocationPermission.denied) {
        return const LocationResult.error(LocationProblem.denied);
      }
      final position = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(
          accuracy: LocationAccuracy.high,
          timeLimit: Duration(seconds: 12),
        ),
      );
      return LocationResult.ok(LatLng(position.latitude, position.longitude));
    } catch (_) {
      return const LocationResult.error(LocationProblem.failed);
    }
  }
}

String locationProblemKey(LocationProblem p) {
  switch (p) {
    case LocationProblem.disabled:
      return 'location.disabled';
    case LocationProblem.denied:
      return 'location.denied';
    case LocationProblem.deniedForever:
      return 'location.denied_forever';
    case LocationProblem.failed:
      return 'location.failed';
  }
}
