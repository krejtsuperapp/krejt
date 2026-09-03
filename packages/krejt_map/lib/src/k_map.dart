/// Harta që përdorin ekranet. Ofruesin e zgjedh konfigurimi; nëse pamja e largët nuk vjen,
/// bihet te skema vendore në vend që të mbetet një kuti bosh.
library;

import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';

import 'config.dart';
import 'mapbox_static.dart';
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

  /// Nën këtë distancë, pamja nuk rikërkohet: shoferi lëviz vazhdimisht, dhe çdo kërkesë
  /// e re është një pamje e faturuar për një ndryshim që syri nuk e dallon.
  static const _refreshAfterMeters = 25.0;

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
              ? _MapboxImage(config: cfg, markers: markers, schematicCaption: schematicCaption)
              : SchematicMap(markers: markers, caption: schematicCaption),
        ),
      ),
    );
  }

  /// Vendos nëse pamja duhet rikërkuar. Krahasimi bëhet me distancë të vërtetë, jo me
  /// rrumbullakosje në rrjetë: dy pika dhjetë metra larg mund të bien në qeliza të ndryshme
  /// dhe do të shkaktonin një kërkesë të re pikërisht aty ku pragu duhej ta ndalonte.
  static List<MapMarker> settle(List<MapMarker> shown, List<MapMarker> next) {
    if (shown.length != next.length) return next;
    for (var i = 0; i < next.length; i++) {
      if (shown[i].kind != next[i].kind) return next;
      if (metersBetween(shown[i].point, next[i].point) > _refreshAfterMeters) return next;
    }
    return shown;
  }

  /// Distancë e mjaftueshme për qytet: gjatësia ngushtohet me gjerësinë, kurbatura shpërfillet.
  static double metersBetween(MapPoint a, MapPoint b) {
    const perDegree = 111320.0;
    final dLat = (b.lat - a.lat) * perDegree;
    final dLng = (b.lng - a.lng) * perDegree * math.cos((a.lat + b.lat) / 2 * math.pi / 180);
    return math.sqrt(dLat * dLat + dLng * dLng);
  }
}

class _MapboxImage extends StatefulWidget {
  const _MapboxImage({required this.config, required this.markers, required this.schematicCaption});

  final MapConfig config;
  final List<MapMarker> markers;
  final String? schematicCaption;

  @override
  State<_MapboxImage> createState() => _MapboxImageState();
}

class _MapboxImageState extends State<_MapboxImage> {
  late List<MapMarker> _shown = widget.markers;

  @override
  void didUpdateWidget(_MapboxImage old) {
    super.didUpdateWidget(old);
    // Shoferi lëviz vazhdimisht; pa këtë, çdo pyetje e serverit do të blinte një pamje të re.
    _shown = KMap.settle(_shown, widget.markers);
  }

  @override
  Widget build(BuildContext context) {
    final dpr = MediaQuery.maybeDevicePixelRatioOf(context) ?? 2;
    return LayoutBuilder(
      builder: (context, constraints) {
        final url =
            MapboxStaticUrl(
              token: widget.config.mapboxToken,
              style: widget.config.mapboxStyle,
            ).build(
              markers: _shown,
              width: math.max(constraints.maxWidth.round(), 1),
              height: math.max(constraints.maxHeight.round(), 1),
              devicePixelRatio: dpr,
            );
        return Image.network(
          url,
          fit: BoxFit.cover,
          gaplessPlayback: true,
          // Pa rrjet ose me çelës të refuzuar, ekrani mbetet i përdorshëm me skemën vendore.
          errorBuilder: (_, _, _) =>
              SchematicMap(markers: widget.markers, caption: widget.schematicCaption),
          loadingBuilder: (context, child, progress) =>
              progress == null ? child : ColoredBox(color: K.surface2, child: child),
        );
      },
    );
  }
}
