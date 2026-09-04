/// Pamja pa rrjet. Pozicionet janë të vërteta dhe përpjesëtimet mes tyre ruhen; rruga
/// vizatohet vetëm kur serveri e ka dhënë gjeometrinë — ndryshe një vijë e ndërprerë thotë
/// hapur se është lidhje, jo rrugë.
library;

import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';

import 'model.dart';

class SchematicMap extends StatelessWidget {
  const SchematicMap({super.key, required this.markers, this.path, this.caption});

  final List<MapMarker> markers;
  final List<MapPoint>? path;
  final String? caption;

  @override
  Widget build(BuildContext context) {
    final hasRoute = (path?.length ?? 0) >= 2;
    return Stack(
      fit: StackFit.expand,
      children: [
        CustomPaint(painter: _SchematicPainter(markers, path)),
        if (caption != null && !hasRoute)
          Positioned(
            left: K.s3,
            bottom: K.s3,
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: K.bg.withValues(alpha: 0.72),
                borderRadius: BorderRadius.circular(K.rSm),
              ),
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: K.s2, vertical: K.s1),
                child: Text(caption!, style: const TextStyle(fontSize: 11, color: K.muted)),
              ),
            ),
          ),
      ],
    );
  }
}

class _SchematicPainter extends CustomPainter {
  _SchematicPainter(this.markers, this.path);

  final List<MapMarker> markers;
  final List<MapPoint>? path;

  static const _colors = {
    MapMarkerKind.pickup: K.brand500,
    MapMarkerKind.dropoff: Color(0xFF2E90FA),
    MapMarkerKind.driver: Color(0xFFFFB020),
    MapMarkerKind.place: K.brand400,
  };

  @override
  void paint(Canvas canvas, Size size) {
    final rect = Offset.zero & size;
    canvas.drawRect(rect, Paint()..color = K.surface2);
    _grid(canvas, size);

    final route = path ?? const <MapPoint>[];
    if (markers.isEmpty && route.isEmpty) return;

    final bounds = MapBounds.around([...markers.map((m) => m.point), ...route]);
    const pad = 28.0;
    final inner = Size(math.max(size.width - pad * 2, 1), math.max(size.height - pad * 2, 1));

    // Gjatësia gjeografike ngushtohet me gjerësinë; pa këtë korrigjim, drejtimi mes dy pikave
    // do të dukej i shtrembër në Kosovë me rreth njëzet e pesë për qind.
    final latSpan = bounds.north - bounds.south;
    final lngSpan = (bounds.east - bounds.west) * math.cos(bounds.center.lat * math.pi / 180);
    final scale = math.min(inner.width / lngSpan, inner.height / latSpan);

    Offset project(MapPoint p) {
      final dx = (p.lng - bounds.center.lng) * math.cos(bounds.center.lat * math.pi / 180) * scale;
      final dy = (p.lat - bounds.center.lat) * scale;
      return Offset(size.width / 2 + dx, size.height / 2 - dy);
    }

    if (route.length >= 2) {
      final line = Path()..moveTo(project(route.first).dx, project(route.first).dy);
      for (final p in route.skip(1)) {
        final o = project(p);
        line.lineTo(o.dx, o.dy);
      }
      canvas.drawPath(
        line,
        Paint()
          ..color = K.bg
          ..style = PaintingStyle.stroke
          ..strokeWidth = 7
          ..strokeCap = StrokeCap.round
          ..strokeJoin = StrokeJoin.round,
      );
      canvas.drawPath(
        line,
        Paint()
          ..color = K.brand500
          ..style = PaintingStyle.stroke
          ..strokeWidth = 4
          ..strokeCap = StrokeCap.round
          ..strokeJoin = StrokeJoin.round,
      );
    } else {
      final pickup = _first(MapMarkerKind.pickup);
      final dropoff = _first(MapMarkerKind.dropoff);
      if (pickup != null && dropoff != null) {
        // Vijë e ndërprerë, jo e plotë: nuk është rruga, është vetëm lidhja mes dy pikave.
        _dashed(canvas, project(pickup.point), project(dropoff.point));
      }
    }

    for (final m in markers) {
      _dot(canvas, project(m.point), _colors[m.kind] ?? K.brand500);
    }
  }

  MapMarker? _first(MapMarkerKind kind) {
    for (final m in markers) {
      if (m.kind == kind) return m;
    }
    return null;
  }

  void _grid(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = K.line
      ..strokeWidth = 1;
    const step = 32.0;
    for (var x = step; x < size.width; x += step) {
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), paint);
    }
    for (var y = step; y < size.height; y += step) {
      canvas.drawLine(Offset(0, y), Offset(size.width, y), paint);
    }
  }

  void _dashed(Canvas canvas, Offset a, Offset b) {
    final paint = Paint()
      ..color = K.line2
      ..strokeWidth = 2
      ..strokeCap = StrokeCap.round;
    final total = (b - a).distance;
    if (total == 0) return;
    final dir = (b - a) / total;
    const dash = 8.0, gap = 6.0;
    for (var t = 0.0; t < total; t += dash + gap) {
      final end = math.min(t + dash, total);
      canvas.drawLine(a + dir * t, a + dir * end, paint);
    }
  }

  void _dot(Canvas canvas, Offset at, Color color) {
    canvas.drawCircle(at, 11, Paint()..color = color.withValues(alpha: 0.22));
    canvas.drawCircle(at, 6, Paint()..color = color);
    canvas.drawCircle(
      at,
      6,
      Paint()
        ..color = K.bg
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2,
    );
  }

  @override
  bool shouldRepaint(_SchematicPainter old) =>
      !sameMarkers(old.markers, markers) || !samePath(old.path, path);
}
