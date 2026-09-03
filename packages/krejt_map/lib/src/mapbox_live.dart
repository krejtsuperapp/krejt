/// Harta interaktive e Mapbox-it. Ekrani nuk e njeh këtë skedar — e zgjedh `KMap`.
library;

import 'package:flutter/material.dart';
import 'package:mapbox_maps_flutter/mapbox_maps_flutter.dart' as mb;

import 'config.dart';
import 'model.dart';
import 'schematic.dart';

class MapboxLiveMap extends StatefulWidget {
  const MapboxLiveMap({
    super.key,
    required this.config,
    required this.markers,
    this.schematicCaption,
  });

  final MapConfig config;
  final List<MapMarker> markers;
  final String? schematicCaption;

  @override
  State<MapboxLiveMap> createState() => _MapboxLiveMapState();
}

class _MapboxLiveMapState extends State<MapboxLiveMap> {
  /// Çelësi vendoset një herë për gjithë procesin; SDK-ja e mban vetë.
  static String? _tokenSet;

  mb.MapboxMap? _map;
  mb.CircleAnnotationManager? _circles;
  bool _failed = false;

  static const _colors = {
    MapMarkerKind.pickup: 0xFF6A3DFF,
    MapMarkerKind.dropoff: 0xFF19C37D,
    MapMarkerKind.driver: 0xFFFFB020,
    MapMarkerKind.place: 0xFF855EFF,
  };

  @override
  void initState() {
    super.initState();
    if (_tokenSet != widget.config.mapboxToken) {
      mb.MapboxOptions.setAccessToken(widget.config.mapboxToken);
      _tokenSet = widget.config.mapboxToken;
    }
  }

  @override
  void didUpdateWidget(MapboxLiveMap old) {
    super.didUpdateWidget(old);
    if (!_sameMarkers(old.markers, widget.markers)) _draw();
  }

  Future<void> _onCreated(mb.MapboxMap map) async {
    _map = map;
    // Elementet e detyrueshme nga kushtet e Mapbox-it mbeten të dukshme; hiqet vetëm
    // shkalla, që nuk i shërben askujt në një hartë kaq të vogël.
    await map.scaleBar.updateSettings(mb.ScaleBarSettings(enabled: false));
    _circles = await map.annotations.createCircleAnnotationManager();
    await _draw();
  }

  Future<void> _draw() async {
    final circles = _circles;
    if (circles == null) return;
    await circles.deleteAll();
    await circles.createMulti([
      for (final m in widget.markers)
        mb.CircleAnnotationOptions(
          geometry: mb.Point(coordinates: mb.Position(m.point.lng, m.point.lat)),
          circleRadius: 7,
          circleColor: _colors[m.kind] ?? 0xFF6A3DFF,
          circleStrokeWidth: 2,
          circleStrokeColor: 0xFF070B18,
        ),
    ]);
    await _frame();
  }

  /// Korniza ndiqet nga kutia e përbashkët, jo nga pikat drejtpërdrejt: kutia e njeh rastin
  /// e një pike të vetme dhe i jep shtrirje minimale, ndaj zmadhimi nuk shkon në pafundësi.
  Future<void> _frame() async {
    final map = _map;
    if (map == null || widget.markers.isEmpty) return;
    final b = MapBounds.around(widget.markers.map((m) => m.point));
    final camera = await map.cameraForCoordinateBounds(
      mb.CoordinateBounds(
        southwest: mb.Point(coordinates: mb.Position(b.west, b.south)),
        northeast: mb.Point(coordinates: mb.Position(b.east, b.north)),
        infiniteBounds: false,
      ),
      mb.MbxEdgeInsets(top: 36, left: 36, bottom: 36, right: 36),
      null,
      null,
      null,
      null,
    );
    await map.flyTo(camera, mb.MapAnimationOptions(duration: 600));
  }

  @override
  Widget build(BuildContext context) {
    if (_failed) {
      return SchematicMap(markers: widget.markers, caption: widget.schematicCaption);
    }
    return mb.MapWidget(
      key: const ValueKey('krejt-map'),
      styleUri: widget.config.mapboxStyleUri,
      onMapCreated: _onCreated,
      // Pa rrjet ose me çelës të refuzuar, ekrani mbetet i përdorshëm me skemën vendore
      // në vend që të mbetet një kuti bosh.
      onMapLoadErrorListener: (_) {
        if (mounted && !_failed) setState(() => _failed = true);
      },
    );
  }
}

bool _sameMarkers(List<MapMarker> a, List<MapMarker> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i].point != b[i].point || a[i].kind != b[i].kind) return false;
  }
  return true;
}
