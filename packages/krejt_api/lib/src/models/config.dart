/// Konfigurimi publik i klientit (§48, §69): versioni minimal, mirëmbajtja dhe flag-et publike.
/// Aplikacioni e merr këtë para çdo ekrani tjetër dhe e ricakton sipas `config_ttl_s`.
library;

class AppVersionInfo {
  AppVersionInfo({
    required this.app,
    required this.platform,
    required this.minVersion,
    required this.recommendedVersion,
    required this.maintenance,
    this.maintenanceMessage,
  });

  final String app;
  final String platform;
  final String minVersion;
  final String recommendedVersion;
  final bool maintenance;
  final String? maintenanceMessage;

  factory AppVersionInfo.fromJson(Map<String, dynamic> j) => AppVersionInfo(
    app: (j['app'] ?? '').toString(),
    platform: (j['platform'] ?? '').toString(),
    minVersion: (j['min_version'] ?? '0.0.0').toString(),
    recommendedVersion: (j['recommended_version'] ?? '0.0.0').toString(),
    maintenance: j['maintenance'] == true,
    maintenanceMessage: j['maintenance_message']?.toString(),
  );
}

enum UpdateState { ok, recommended, updateRequired, maintenance, unknown }

class PublicConfig {
  PublicConfig({
    this.app,
    required this.updateState,
    required this.flags,
    required this.serverTime,
    required this.configTtlS,
  });

  final AppVersionInfo? app;
  final UpdateState updateState;
  final Map<String, bool> flags;
  final DateTime serverTime;
  final int configTtlS;

  bool flag(String key, {bool fallback = false}) => flags[key] ?? fallback;

  /// Bllokon hyrjen: versioni nën minimumin ose mirëmbajtje e planifikuar.
  bool get blocksApp =>
      updateState == UpdateState.updateRequired || updateState == UpdateState.maintenance;

  static PublicConfig fallback() => PublicConfig(
    updateState: UpdateState.unknown,
    flags: const {},
    serverTime: DateTime.now(),
    configTtlS: 60,
  );

  factory PublicConfig.fromJson(Map<String, dynamic> j) {
    final raw = (j['update_state'] ?? 'unknown').toString();
    final state = UpdateState.values.firstWhere(
      (s) => s.name == raw || (s == UpdateState.updateRequired && raw == 'required'),
      orElse: () => UpdateState.unknown,
    );
    final flags = <String, bool>{};
    final f = j['flags'];
    if (f is Map) {
      f.forEach((k, v) => flags[k.toString()] = v == true);
    }
    return PublicConfig(
      app: j['app'] is Map
          ? AppVersionInfo.fromJson(Map<String, dynamic>.from(j['app'] as Map))
          : null,
      updateState: state,
      flags: flags,
      serverTime: DateTime.tryParse(j['server_time']?.toString() ?? '') ?? DateTime.now(),
      configTtlS: (j['config_ttl_s'] as num?)?.toInt() ?? 300,
    );
  }
}
