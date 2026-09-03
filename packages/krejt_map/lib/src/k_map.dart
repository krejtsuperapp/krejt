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
    this.config,
    this.height = 200,
    this.schematicCaption,
    this.semanticsLabel,
  });

  final List<MapMarker> markers;

  /// Zakonisht lihet bosh dhe merret nga `--dart-define`; testet e japin vetë.
  final MapConfig? config;

  final double height;

  /// Teksti që shpjegon se skema nuk tregon rrugë. Vjen nga ekrani, sepse harta nuk i njeh gjuhët.
  final String? schematicCaption;

  final String? semanticsLabel;

  @override
  Widget build(BuildContext context) {
    final cfg = config ?? MapConfig.fromEnv();
    return Semantics(
      label: semanticsLabel,
      image: true,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(K.rMd),
        child: SizedBox(
          height: height,
          width: double.infinity,
          child: cfg.kind == MapProviderKind.mapbox
              ? MapboxLiveMap(config: cfg, markers: markers, schematicCaption: schematicCaption)
              : SchematicMap(markers: markers, caption: schematicCaption),
        ),
      ),
    );
  }
}
