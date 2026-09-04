/// Marka: fjala KREJT me E-në prej tri vijash horizontale (pa shtyllë), dhe ndërruesi i gjuhës.
/// Të dyja jetojnë këtu që splash-i, hyrja dhe kyçja ta vizatojnë të njëjtën logo.
library;

import 'package:flutter/material.dart';

import '../tokens.dart';

/// Logoja KREJT. Shkronja E vizatohet si tri vija neon që hyjnë me radhë nga e majta;
/// pas hyrjes, vijat pulsojnë lehtë. Me `animate: false` shfaqet e qetë (koka e ekraneve).
class KWordmark extends StatefulWidget {
  const KWordmark({
    super.key,
    this.size = 44,
    this.animate = true,
    this.color = K.text,
    this.barColor = K.brand500,
  });

  /// Madhësia e shkronjave (fontSize). Vijat e E-së ndjekin lartësinë e kapitaleve.
  final double size;
  final bool animate;
  final Color color;
  final Color barColor;

  @override
  State<KWordmark> createState() => _KWordmarkState();
}

class _KWordmarkState extends State<KWordmark> with TickerProviderStateMixin {
  late final AnimationController _enter = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1100),
  );
  late final AnimationController _pulse = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1600),
  );

  @override
  void initState() {
    super.initState();
    if (widget.animate) {
      _enter.forward().whenComplete(() {
        if (mounted) _pulse.repeat(reverse: true);
      });
    } else {
      _enter.value = 1;
    }
  }

  @override
  void dispose() {
    _enter.dispose();
    _pulse.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final reduce = MediaQuery.maybeDisableAnimationsOf(context) ?? false;
    if (reduce && widget.animate && !_enter.isCompleted) _enter.value = 1;
    final s = widget.size;
    final style = TextStyle(
      fontFamily: K.fontFamily,
      package: K.fontPackage,
      fontSize: s,
      fontWeight: FontWeight.w800,
      letterSpacing: s * 0.08,
      height: 1,
      color: widget.color,
    );
    return AnimatedBuilder(
      animation: Listenable.merge([_enter, _pulse]),
      builder: (context, _) {
        final letters = CurvedAnimation(
          parent: _enter,
          curve: const Interval(0, 0.45, curve: Curves.easeOutCubic),
        ).value;
        return Row(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            _Fade(
              t: letters,
              child: Text('KR', style: style),
            ),
            SizedBox(width: s * 0.06),
            _BarsE(size: s, color: widget.barColor, enter: _enter.value, pulse: _pulse.value),
            SizedBox(width: s * 0.14),
            _Fade(
              t: letters,
              child: Text('JT', style: style),
            ),
          ],
        );
      },
    );
  }
}

class _Fade extends StatelessWidget {
  const _Fade({required this.t, required this.child});

  final double t;
  final Widget child;

  @override
  Widget build(BuildContext context) => Opacity(
    opacity: t,
    child: Transform.translate(offset: Offset(0, (1 - t) * 6), child: child),
  );
}

/// Tri vijat e E-së. Secila hyn nga e majta me vonesë të vetën (0.35 → 0.95 e hyrjes).
class _BarsE extends StatelessWidget {
  const _BarsE({required this.size, required this.color, required this.enter, required this.pulse});

  final double size;
  final Color color;
  final double enter;
  final double pulse;

  @override
  Widget build(BuildContext context) {
    final capH = size * 0.72;
    final w = size * 0.58;
    final t = size * 0.14;
    final glow = 0.35 + 0.4 * pulse;
    return SizedBox(
      width: w,
      height: capH,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          for (var i = 0; i < 3; i++)
            _Bar(
              width: i == 1 ? w * 0.8 : w,
              height: t,
              color: color,
              scale: _stagger(enter, i),
              glow: glow,
            ),
        ],
      ),
    );
  }

  static double _stagger(double t, int i) {
    final start = 0.35 + i * 0.18;
    final end = start + 0.28;
    final v = ((t - start) / (end - start)).clamp(0.0, 1.0);
    return Curves.easeOutBack.transform(v).clamp(0.0, 1.15);
  }
}

class _Bar extends StatelessWidget {
  const _Bar({
    required this.width,
    required this.height,
    required this.color,
    required this.scale,
    required this.glow,
  });

  final double width;
  final double height;
  final Color color;
  final double scale;
  final double glow;

  @override
  Widget build(BuildContext context) => Align(
    alignment: Alignment.centerLeft,
    child: Transform.scale(
      scaleX: scale,
      alignment: Alignment.centerLeft,
      child: Container(
        width: width,
        height: height,
        decoration: BoxDecoration(
          color: color,
          borderRadius: BorderRadius.circular(height),
          boxShadow: [
            BoxShadow(
              color: color.withValues(alpha: glow),
              blurRadius: height * 1.6,
              spreadRadius: height * 0.1,
            ),
          ],
        ),
      ),
    ),
  );
}

/// Ndërruesi i gjuhës: SQ · EN · DE në një pilulë. Segmenti i zgjedhur ndriçon neon.
class KLangSwitch extends StatelessWidget {
  const KLangSwitch({
    super.key,
    required this.value,
    required this.onChanged,
    this.codes = const ['sq', 'en', 'de'],
  });

  final String value;
  final ValueChanged<String> onChanged;
  final List<String> codes;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(3),
    decoration: BoxDecoration(
      color: K.surface.withValues(alpha: 0.85),
      borderRadius: BorderRadius.circular(K.rFull),
      border: Border.all(color: K.line2),
    ),
    child: Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final c in codes)
          Semantics(
            button: true,
            selected: c == value,
            label: c.toUpperCase(),
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: () => onChanged(c),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 220),
                curve: Curves.easeOut,
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                decoration: BoxDecoration(
                  color: c == value ? K.brand500 : Colors.transparent,
                  borderRadius: BorderRadius.circular(K.rFull),
                  boxShadow: c == value
                      ? [BoxShadow(color: K.brand500.withValues(alpha: 0.45), blurRadius: 12)]
                      : null,
                ),
                child: Text(
                  c.toUpperCase(),
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w800,
                    letterSpacing: 0.8,
                    color: c == value ? K.onBrand : K.textDim,
                  ),
                ),
              ),
            ),
          ),
      ],
    ),
  );
}
