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
    this.path,
    this.padding = const EdgeInsets.all(36),
    this.interactive = false,
    this.schematicCaption,
    this.showUserLocation = false,
    this.recenterTooltip,
  });

  final MapConfig config;
  final List<MapMarker> markers;
  final List<MapPoint>? path;
  final EdgeInsets padding;
  final bool interactive;
  final String? schematicCaption;
  final bool showUserLocation;
  final String? recenterTooltip;

  @override
  State<MapboxLiveMap> createState() => _MapboxLiveMapState();
}

class _MapboxLiveMapState extends State<MapboxLiveMap> {
  /// Çelësi vendoset një herë për gjithë procesin; SDK-ja e mban vetë.
  static String? _tokenSet;

  mb.MapboxMap? _map;
  mb.CircleAnnotationManager? _circles;
  mb.PolylineAnnotationManager? _lines;
  bool _failed = false;

  /// Rruga e vizatuar e fundit. Pa këtë, çdo lëvizje e shoferit e fshinte dhe e rivizatonte
  /// polilinjën — dridhje e dukshme çdo dy sekonda, dhe punë e kotë.
  List<MapPoint>? _drawnPath;

  /// Kur përdoruesi e ka zvarritur hartën, korniza nuk i rrëmbehet më vetvetiu: rikthehet vetëm
  /// kur e kërkon ai me butonin.
  bool _userMoved = false;

  /// Korniza ndiqet vetëm kur ndryshojnë pikat fikse ose rruga — jo sa herë lëviz shoferi,
  /// që harta të mos kërcejë çdo dy sekonda.
  String _framed = '';

  // Marka: neoni për pikën e marrjes dhe vendet; dorëzimi blu dhe shoferi qelibar, që tri pikat
  // të dallohen edhe mbi hartë të errët. Konturi i zi si sfondi i markës (#0D0D0D).
  static const _colors = {
    MapMarkerKind.pickup: 0xFF39FF14,
    MapMarkerKind.dropoff: 0xFF2E90FA,
    MapMarkerKind.driver: 0xFFFFB020,
    MapMarkerKind.place: 0xFF5CFF3D,
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
    if (!sameMarkers(old.markers, widget.markers) ||
        !samePath(old.path, widget.path) ||
        old.padding != widget.padding) {
      _draw();
    }
  }

  Future<void> _onCreated(mb.MapboxMap map) async {
    _map = map;
    // Elementet e detyrueshme nga kushtet e Mapbox-it mbeten të dukshme; hiqen vetëm
    // shkalla dhe busulla, që nuk i shërbejnë askujt këtu.
    await map.scaleBar.updateSettings(mb.ScaleBarSettings(enabled: false));
    await map.compass.updateSettings(mb.CompassSettings(enabled: false));
    await map.gestures.updateSettings(
      widget.interactive
          ? mb.GesturesSettings(rotateEnabled: false, pitchEnabled: false)
          : mb.GesturesSettings(
              scrollEnabled: false,
              pinchToZoomEnabled: false,
              rotateEnabled: false,
              pitchEnabled: false,
              doubleTapToZoomInEnabled: false,
              doubleTouchToZoomOutEnabled: false,
            ),
    );
    // Pika e përdoruesit ndizet vetëm kur ekrani e kërkon: ndezja i kërkon sistemit lejen e
    // vendndodhjes, dhe ajo duhet kërkuar kur shërbimi e do vërtet.
    if (widget.showUserLocation) {
      await map.location.updateSettings(
        mb.LocationComponentSettings(enabled: true, pulsingEnabled: true, showAccuracyRing: true),
      );
    }
    _lines = await map.annotations.createPolylineAnnotationManager();
    _circles = await map.annotations.createCircleAnnotationManager();
    await _draw();
  }

  Future<void> _draw() async {
    await _drawPath();
    await _drawMarkers();
    await _frame();
  }

  /// Rruga rivizatohet vetëm kur ndryshon vërtet.
  Future<void> _drawPath() async {
    final lines = _lines;
    if (lines == null) return;
    final path = widget.path;
    if (samePath(_drawnPath, path)) return;
    _drawnPath = path;
    await lines.deleteAll();
    if (path != null && path.length >= 2) {
      final coords = [for (final p in path) mb.Position(p.lng, p.lat)];
      // Dy shtresa: një kontur i errët poshtë dhe neoni sipër — lexohet mbi çdo rrugë.
      await lines.createMulti([
        mb.PolylineAnnotationOptions(
          geometry: mb.LineString(coordinates: coords),
          lineColor: 0xFF0D0D0D,
          lineWidth: 7,
          lineOpacity: 0.85,
        ),
        mb.PolylineAnnotationOptions(
          geometry: mb.LineString(coordinates: coords),
          lineColor: 0xFF39FF14,
          lineWidth: 4,
        ),
      ]);
    }
  }

  Future<void> _drawMarkers() async {
    final circles = _circles;
    if (circles == null) return;
    await circles.deleteAll();
    await circles.createMulti([
      for (final m in widget.markers)
        mb.CircleAnnotationOptions(
          geometry: mb.Point(coordinates: mb.Position(m.point.lng, m.point.lat)),
          circleRadius: m.kind == MapMarkerKind.driver ? 8 : 7,
          circleColor: _colors[m.kind] ?? 0xFF39FF14,
          circleStrokeWidth: 2,
          circleStrokeColor: 0xFF0D0D0D,
        ),
    ]);
  }

  /// Korniza ndiqet nga kutia e përbashkët, jo nga pikat drejtpërdrejt: kutia e njeh rastin
  /// e një pike të vetme dhe i jep shtrirje minimale, ndaj zmadhimi nuk shkon në pafundësi.
  Future<void> _frame({bool force = false}) async {
    final map = _map;
    if (map == null) return;
    final fixed = widget.markers.where((m) => m.kind != MapMarkerKind.driver).map((m) => m.point);
    final pts = [...fixed, ...?widget.path];
    if (pts.isEmpty) return;
    final key = '${pts.first}|${pts.last}|${pts.length}|${widget.padding}';
    if (key == _framed && !force) return;
    if (_userMoved && !force) return;
    _framed = key;
    final b = MapBounds.around(pts);
    final camera = await map.cameraForCoordinateBounds(
      mb.CoordinateBounds(
        southwest: mb.Point(coordinates: mb.Position(b.west, b.south)),
        northeast: mb.Point(coordinates: mb.Position(b.east, b.north)),
        infiniteBounds: false,
      ),
      mb.MbxEdgeInsets(
        top: widget.padding.top,
        left: widget.padding.left,
        bottom: widget.padding.bottom,
        right: widget.padding.right,
      ),
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
      return SchematicMap(
        markers: widget.markers,
        path: widget.path,
        caption: widget.schematicCaption,
      );
    }
    final map = mb.MapWidget(
      key: const ValueKey('krejt-map'),
      styleUri: widget.config.mapboxStyleUri,
      onMapCreated: _onCreated,
      // Pa rrjet ose me çelës të refuzuar, ekrani mbetet i përdorshëm me skemën vendore
      // në vend që të mbetet një kuti bosh.
      onMapLoadErrorListener: (_) {
        if (mounted && !_failed) setState(() => _failed = true);
      },
      // Sapo përdoruesi e zvarrit hartën vetë, korniza nuk ia rrëmben më: një hartë që kthehet
      // vetvetiu sa herë lëviz shoferi nuk lejon të shohësh asgjë tjetër.
      onScrollListener: widget.interactive
          ? (_) {
              if (!_userMoved) _userMoved = true;
            }
          : null,
    );
    final tooltip = widget.recenterTooltip;
    if (tooltip == null) return map;
    return Stack(
      children: [
        Positioned.fill(child: map),
        Positioned(
          right: 12,
          bottom: widget.padding.bottom + 12,
          child: _RecenterButton(
            tooltip: tooltip,
            onPressed: () {
              _userMoved = false;
              _frame(force: true);
            },
          ),
        ),
      ],
    );
  }
}

/// Butoni që rikthen kornizën te rruga. Pa të, një zvarritje e vetme e linte përdoruesin pa
/// mënyrë për t'u kthyer — dhe harta nuk ka "mbrapa".
class _RecenterButton extends StatelessWidget {
  const _RecenterButton({required this.tooltip, required this.onPressed});

  final String tooltip;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) => Semantics(
    button: true,
    label: tooltip,
    child: Material(
      color: const Color(0xE60D0D0D),
      shape: const CircleBorder(),
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onPressed,
        child: const SizedBox(
          width: 44,
          height: 44,
          child: Icon(Icons.my_location, size: 20, color: Color(0xFF39FF14)),
        ),
      ),
    ),
  );
}
