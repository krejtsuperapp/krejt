/// Harta që përdorin ekranet. Ofruesin e zgjedh konfigurimi; nëse harta e Mapbox-it nuk
/// ngarkohet, bihet te skema vendore në vend që të mbetet një kuti bosh.
library;

import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';

import 'config.dart';
import 'mapbox_live.dart';
import 'model.dart';
import 'schematic.dart';

class KMap extends StatelessWidget {
  const KMap({
    super.key,
    required this.markers,
    this.path,
    this.config,
    this.height = 200,
    this.padding = const EdgeInsets.all(36),
    this.interactive = false,
    this.rounded = true,
    this.schematicCaption,
    this.semanticsLabel,
  });

  final List<MapMarker> markers;

  /// Gjeometria e rrugës (nga serveri). Pa të, skema vizaton vetëm lidhjen e ndërprerë.
  final List<MapPoint>? path;

  /// Zakonisht lihet bosh dhe merret nga `--dart-define`; testet e japin vetë.
  final MapConfig? config;

  /// Lartësia e kartës; `null` = mbush hapësirën që i jep prindi (ekranet me hartë të plotë).
  final double? height;

  /// Hapësira që lihet bosh kur korniza ndjek pikat — poshtë vendoset lartësia e fletës.
  final EdgeInsets padding;

  /// Zvarritje/zmadhim me gisht. Kartat e vogla e lënë të fikur, që lista të rrëshqasë.
  final bool interactive;

  final bool rounded;

  /// Teksti që shpjegon se skema nuk tregon rrugë. Vjen nga ekrani, sepse harta nuk i njeh gjuhët.
  final String? schematicCaption;

  final String? semanticsLabel;

  @override
  Widget build(BuildContext context) {
    final cfg = config ?? MapConfig.fromEnv();
    final child = cfg.kind == MapProviderKind.mapbox
        ? MapboxLiveMap(
            config: cfg,
            markers: markers,
            path: path,
            padding: padding,
            interactive: interactive,
            schematicCaption: schematicCaption,
          )
        : SchematicMap(markers: markers, path: path, caption: schematicCaption);
    return Semantics(
      label: semanticsLabel,
      image: true,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(rounded ? K.rMd : 0),
        child: SizedBox(height: height, width: double.infinity, child: child),
      ),
    );
  }
}
