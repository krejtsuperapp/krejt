/// Zgjedhja e ofruesit, e njëjta rrjedhë si te serveri: një emër vendos, dhe pa çelës bihet
/// te varianti pa rrjet. Asnjë çelës nuk qëndron te kodi — vjen me `--dart-define` gjatë ndërtimit.
library;

enum MapProviderKind {
  /// Pamje nga Mapbox-i (Static Images API): rrugë të vërteta, pa SDK vendase.
  mapbox,

  /// Skemë vendore kur ofruesi mungon: pozicionet janë të vërteta, rrugët nuk vizatohen.
  schematic,
}

class MapConfig {
  const MapConfig({required this.kind, this.mapboxToken = '', this.mapboxStyle = _defaultStyle});

  static const _defaultStyle = 'mapbox/dark-v11';

  final MapProviderKind kind;
  final String mapboxToken;
  final String mapboxStyle;

  /// `KREJT_MAP_PROVIDER` zgjedh, `KREJT_MAPBOX_TOKEN` jep çelësin publik (`pk.`).
  /// Pa çelës, Mapbox-i nuk zgjidhet dot: më mirë një skemë e sinqertë se një kuti bosh.
  factory MapConfig.fromEnv({
    String provider = const String.fromEnvironment('KREJT_MAP_PROVIDER'),
    String token = const String.fromEnvironment('KREJT_MAPBOX_TOKEN'),
    String style = const String.fromEnvironment('KREJT_MAPBOX_STYLE'),
  }) {
    final wantsMapbox = provider.isEmpty ? token.isNotEmpty : provider == 'mapbox';
    if (wantsMapbox && token.isNotEmpty) {
      return MapConfig(
        kind: MapProviderKind.mapbox,
        mapboxToken: token,
        mapboxStyle: style.isEmpty ? _defaultStyle : style,
      );
    }
    return const MapConfig(kind: MapProviderKind.schematic);
  }

  bool get isLive => kind != MapProviderKind.schematic;

  /// SDK-ja e kërkon stilin si adresë `mapbox://`; konfigurimi e mban emrin e shkurtër,
  /// që `--dart-define` të mbetet i lexueshëm. Një adresë e plotë kalon e paprekur.
  String get mapboxStyleUri =>
      mapboxStyle.contains('://') ? mapboxStyle : 'mapbox://styles/$mapboxStyle';
}
