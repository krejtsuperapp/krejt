import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';

/// Ekran me hartë të plotë poshtë dhe një panel mbi të. Dy variante:
/// - [MapScaffold.panel]: paneli qëndron poshtë me lartësi të vetën (zgjedhje, ofertë);
/// - [MapScaffold.sheet]: fletë që tërhiqet lart (ndjekja e udhëtimit/porosisë).
/// Harta lë hapësirë poshtë sa paneli, që rruga të mos fshihet nën të.
class MapScaffold extends StatelessWidget {
  const MapScaffold.panel({
    super.key,
    required this.title,
    required this.markers,
    required this.panel,
    this.path,
    this.actions,
  }) : sheet = null,
       sheetInitial = 0;

  const MapScaffold.sheet({
    super.key,
    required this.title,
    required this.markers,
    required this.sheet,
    this.sheetInitial = 0.46,
    this.path,
    this.actions,
  }) : panel = null;

  final String title;
  final List<MapMarker> markers;
  final List<MapPoint>? path;
  final List<Widget>? actions;

  /// Paneli fiks (variant `panel`).
  final Widget? panel;

  /// Përmbajtja e fletës që tërhiqet (variant `sheet`); merr kontrolluesin e rrëshqitjes.
  final Widget Function(ScrollController controller)? sheet;
  final double sheetInitial;

  @override
  Widget build(BuildContext context) {
    final h = MediaQuery.sizeOf(context).height;
    final bottomPad = panel != null ? h * 0.5 : h * sheetInitial + 24;
    return Scaffold(
      backgroundColor: K.bg,
      extendBodyBehindAppBar: true,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        elevation: 0,
        title: Text(title),
        leading: _Glass(
          child: IconButton(
            icon: const Icon(Icons.arrow_back, color: K.text),
            tooltip: MaterialLocalizations.of(context).backButtonTooltip,
            onPressed: () => Navigator.of(context).maybePop(),
          ),
        ),
        actions: actions,
      ),
      body: Stack(
        fit: StackFit.expand,
        children: [
          KMap(
            height: null,
            rounded: false,
            interactive: true,
            markers: markers,
            path: path,
            padding: EdgeInsets.fromLTRB(48, 120, 48, bottomPad),
            // Kur harta e vërtetë nuk ngarkohet, teksti e thotë hapur se kjo është skemë dhe jo hartë.
            schematicCaption: context.t('map.schematic'),
          ),
          // Hija nën kokën transparente, që titulli të lexohet mbi hartë të çelët.
          const Positioned(
            left: 0,
            right: 0,
            top: 0,
            height: 120,
            child: IgnorePointer(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [Color(0xCC0D0D0D), Color(0x000D0D0D)],
                  ),
                ),
              ),
            ),
          ),
          if (panel != null)
            Positioned(left: 0, right: 0, bottom: 0, child: _PanelBox(child: panel!))
          else
            DraggableScrollableSheet(
              initialChildSize: sheetInitial,
              minChildSize: 0.22,
              maxChildSize: 0.92,
              snap: true,
              builder: (context, controller) => _PanelBox(child: sheet!(controller)),
            ),
        ],
      ),
    );
  }
}

class _PanelBox extends StatelessWidget {
  const _PanelBox({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) => Container(
    decoration: BoxDecoration(
      color: K.surface,
      border: const Border(top: BorderSide(color: K.line)),
      borderRadius: const BorderRadius.vertical(top: Radius.circular(K.rXl)),
      boxShadow: [
        BoxShadow(color: K.bg.withValues(alpha: 0.6), blurRadius: 24, offset: const Offset(0, -6)),
      ],
    ),
    child: child,
  );
}

class _Glass extends StatelessWidget {
  const _Glass({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) => Center(
    child: Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        color: K.surface.withValues(alpha: 0.85),
        borderRadius: BorderRadius.circular(K.rFull),
        border: Border.all(color: K.line2),
      ),
      child: child,
    ),
  );
}

/// Dy rreshta nisje/destinacion me shenjat e hartës (neon = nisja, blu = destinacioni).
class RouteEnds extends StatelessWidget {
  const RouteEnds({super.key, required this.pickup, required this.dropoff});

  final String pickup;
  final String dropoff;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      _End(color: K.brand500, label: pickup, square: false),
      Padding(
        padding: const EdgeInsets.only(left: 5),
        child: Container(width: 2, height: 14, color: K.line2),
      ),
      _End(color: K.info, label: dropoff, square: true),
    ],
  );
}

class _End extends StatelessWidget {
  const _End({required this.color, required this.label, required this.square});

  final Color color;
  final String label;
  final bool square;

  @override
  Widget build(BuildContext context) => Row(
    children: [
      Container(
        width: 12,
        height: 12,
        decoration: BoxDecoration(
          color: color,
          borderRadius: BorderRadius.circular(square ? 2 : K.rFull),
          boxShadow: [BoxShadow(color: color.withValues(alpha: 0.5), blurRadius: 8)],
        ),
      ),
      const SizedBox(width: K.s3),
      Expanded(
        child: Text(
          label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: K.text),
        ),
      ),
    ],
  );
}

MapMarker markerOf(double lat, double lng, MapMarkerKind kind) =>
    MapMarker(point: MapPoint(lat, lng), kind: kind);
