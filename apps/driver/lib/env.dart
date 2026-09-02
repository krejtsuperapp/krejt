/// Cilësimet e ndërtimit. Asnjë çelës sekret nuk qëndron këtu — vetëm adresa e API-së,
/// e dhënë me `--dart-define` gjatë ndërtimit (§69).
class Env {
  static const apiBaseUrl = String.fromEnvironment(
    'KREJT_API_BASE_URL',
    defaultValue: 'http://10.0.2.2:8080',
  );

  static const appVersion = String.fromEnvironment('KREJT_APP_VERSION', defaultValue: '1.0.0');

  static const appId = 'driver';
}
