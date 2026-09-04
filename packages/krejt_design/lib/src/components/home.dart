import 'dart:async';

import 'package:flutter/material.dart';

import '../tokens.dart';
import 'images.dart';

/// Komponentët e Ballinës sipas mockup-it të markës (04.09.2026): slide fotosh me ofertë,
/// pllaka shërbimesh me ikonë neon, karta vendesh me foto, pill-a, targë dhe KPI.
/// Të gjithë lexojnë vetëm tokenët: asnjë ngjyrë e ngurtë jashtë `K`.

/// Një slide i karuselit: foto, etiketë e vogël neon, titull dhe veprim.
class KHeroSlide {
  const KHeroSlide({
    required this.title,
    this.tag,
    this.imageUrl,
    this.assetImage,
    this.actionLabel,
    this.onTap,
  });

  final String title;
  final String? tag;
  final String? imageUrl;

  /// Foto e paketuar me aplikacionin; përdoret kur nuk ka ende foto nga serveri.
  final String? assetImage;
  final String? actionLabel;
  final VoidCallback? onTap;
}

/// Karuseli i ofertave: ndërrohet vetë çdo [every], me pika neon që tregojnë ku je.
/// Pa foto, slide-i mban sfond të errët me gradientin e markës, kurrë një kuti bosh.
class KHeroCarousel extends StatefulWidget {
  const KHeroCarousel({
    super.key,
    required this.slides,
    this.height = 172,
    this.every = const Duration(seconds: 5),
  });

  final List<KHeroSlide> slides;
  final double height;
  final Duration every;

  @override
  State<KHeroCarousel> createState() => _KHeroCarouselState();
}

class _KHeroCarouselState extends State<KHeroCarousel> {
  final _controller = PageController();
  Timer? _timer;
  int _index = 0;

  @override
  void initState() {
    super.initState();
    _schedule();
  }

  @override
  void didUpdateWidget(covariant KHeroCarousel old) {
    super.didUpdateWidget(old);
    if (old.slides.length != widget.slides.length) {
      _index = 0;
      _schedule();
    }
  }

  void _schedule() {
    _timer?.cancel();
    if (widget.slides.length < 2) return;
    _timer = Timer.periodic(widget.every, (_) {
      if (!mounted || !_controller.hasClients) return;
      final next = (_index + 1) % widget.slides.length;
      _controller.animateToPage(
        next,
        duration: const Duration(milliseconds: 520),
        curve: Curves.easeOutCubic,
      );
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (widget.slides.isEmpty) return const SizedBox.shrink();
    return SizedBox(
      height: widget.height,
      child: Stack(
        children: [
          PageView.builder(
            controller: _controller,
            itemCount: widget.slides.length,
            onPageChanged: (i) {
              setState(() => _index = i);
              _schedule();
            },
            itemBuilder: (_, i) => _Slide(slide: widget.slides[i]),
          ),
          if (widget.slides.length > 1)
            Positioned(
              right: 14,
              bottom: 14,
              child: Row(
                children: [
                  for (var i = 0; i < widget.slides.length; i++)
                    AnimatedContainer(
                      duration: const Duration(milliseconds: 300),
                      margin: const EdgeInsets.only(left: 5),
                      width: i == _index ? 16 : 6,
                      height: 6,
                      decoration: BoxDecoration(
                        color: i == _index ? K.brand500 : K.text.withValues(alpha: 0.35),
                        borderRadius: BorderRadius.circular(3),
                        boxShadow: i == _index
                            ? [BoxShadow(color: K.brand500.withValues(alpha: 0.6), blurRadius: 8)]
                            : null,
                      ),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _Slide extends StatelessWidget {
  const _Slide({required this.slide});

  final KHeroSlide slide;

  @override
  Widget build(BuildContext context) {
    final url = slide.imageUrl;
    return ClipRRect(
      borderRadius: BorderRadius.circular(22),
      child: Material(
        color: K.surface,
        child: InkWell(
          onTap: slide.onTap,
          child: Stack(
            fit: StackFit.expand,
            children: [
              if (url != null && url.isNotEmpty)
                KNetImage(url: url, radius: 0, fallbackIcon: Icons.storefront_outlined)
              else if (slide.assetImage != null)
                Image.asset(
                  slide.assetImage!,
                  fit: BoxFit.cover,
                  gaplessPlayback: true,
                  excludeFromSemantics: true,
                )
              else
                const DecoratedBox(
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      begin: Alignment.topLeft,
                      end: Alignment.bottomRight,
                      colors: [K.brand100, K.surface],
                    ),
                  ),
                ),
              DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.centerLeft,
                    end: Alignment.centerRight,
                    stops: const [0, 0.55, 1],
                    colors: [
                      K.bg.withValues(alpha: 0.92),
                      K.bg.withValues(alpha: 0.55),
                      K.bg.withValues(alpha: 0.05),
                    ],
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.all(18),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (slide.tag != null)
                          Text(
                            slide.tag!.toUpperCase(),
                            style: const TextStyle(
                              fontSize: 11,
                              fontWeight: FontWeight.w700,
                              letterSpacing: 1.3,
                              color: K.brand500,
                            ),
                          ),
                        const SizedBox(height: 6),
                        ConstrainedBox(
                          constraints: const BoxConstraints(maxWidth: 240),
                          child: Text(
                            slide.title,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                              fontSize: 22,
                              fontWeight: FontWeight.w800,
                              letterSpacing: -0.4,
                              height: 1.1,
                              color: K.text,
                            ),
                          ),
                        ),
                      ],
                    ),
                    if (slide.actionLabel != null)
                      KNeonChipButton(label: slide.actionLabel!, onTap: slide.onTap),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Buton i vogël neon me shkëlqim (për brenda kartave dhe slide-eve).
class KNeonChipButton extends StatelessWidget {
  const KNeonChipButton({super.key, required this.label, this.onTap});

  final String label;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => Material(
    color: Colors.transparent,
    child: InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(11),
      child: Ink(
        height: 36,
        padding: const EdgeInsets.symmetric(horizontal: 14),
        decoration: BoxDecoration(
          color: K.brand500,
          borderRadius: BorderRadius.circular(11),
          boxShadow: [BoxShadow(color: K.brand500.withValues(alpha: 0.35), blurRadius: 18)],
        ),
        child: Center(
          child: Text(
            label,
            style: const TextStyle(color: K.onBrand, fontWeight: FontWeight.w700, fontSize: 13),
          ),
        ),
      ),
    ),
  );
}

/// Pllakë shërbimi: ikonë neon mbi sfond jeshil të errët, emër poshtë. E fikur kur nuk është gati.
class KServiceTile extends StatelessWidget {
  const KServiceTile({
    super.key,
    required this.icon,
    required this.label,
    this.ready = true,
    this.soonLabel,
    this.onTap,
  });

  final IconData icon;
  final String label;
  final bool ready;
  final String? soonLabel;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => Material(
    color: K.surface,
    borderRadius: BorderRadius.circular(18),
    child: InkWell(
      onTap: ready ? onTap : null,
      borderRadius: BorderRadius.circular(18),
      child: Ink(
        padding: const EdgeInsets.fromLTRB(10, 14, 10, 12),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(18),
          border: Border.all(color: K.line),
        ),
        child: Column(
          children: [
            Container(
              width: 46,
              height: 46,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(15),
                gradient: ready
                    ? const LinearGradient(
                        begin: Alignment.topCenter,
                        end: Alignment.bottomCenter,
                        colors: [Color(0xFF153A10), K.brand50],
                      )
                    : null,
                color: ready ? null : K.surface2,
                border: Border.all(color: ready ? K.brand500.withValues(alpha: 0.28) : K.line),
                boxShadow: ready
                    ? [BoxShadow(color: K.brand500.withValues(alpha: 0.08), blurRadius: 16)]
                    : null,
              ),
              child: Icon(icon, size: 26, color: ready ? K.brand500 : K.muted),
            ),
            const SizedBox(height: 10),
            Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 12.5,
                fontWeight: FontWeight.w600,
                color: ready ? K.textDim : K.muted,
              ),
            ),
            if (!ready && soonLabel != null)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(soonLabel!, style: const TextStyle(fontSize: 10.5, color: K.muted)),
              ),
          ],
        ),
      ),
    ),
  );
}

/// Pill i vogël me tekst; neon kur është "aktiv/hapur".
class KPill extends StatelessWidget {
  const KPill(this.label, {super.key, this.icon, this.neon = false});

  final String label;
  final IconData? icon;
  final bool neon;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 7),
    decoration: BoxDecoration(
      color: neon ? K.brand500.withValues(alpha: 0.08) : K.surface,
      borderRadius: BorderRadius.circular(K.rFull),
      border: Border.all(color: neon ? K.brand500.withValues(alpha: 0.35) : K.line2),
    ),
    child: Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (icon != null) ...[
          Icon(icon, size: 14, color: neon ? K.brand500 : K.textDim),
          const SizedBox(width: 5),
        ],
        Text(
          label,
          style: TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.w600,
            color: neon ? K.brand500 : K.textDim,
          ),
        ),
      ],
    ),
  );
}

/// Targa e automjetit: e bardhë me shkronja të zeza, si e vërteta.
class KPlate extends StatelessWidget {
  const KPlate(this.plate, {super.key});

  final String plate;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
    decoration: BoxDecoration(
      color: K.text,
      borderRadius: BorderRadius.circular(8),
      border: Border.all(color: K.line2),
    ),
    child: Text(
      plate,
      style: const TextStyle(
        color: K.onBrand,
        fontSize: 13,
        fontWeight: FontWeight.w800,
        letterSpacing: 0.8,
      ),
    ),
  );
}

/// Kartë vendi me foto, pill vlerësimi mbi foto, emër dhe dy rreshta meta.
class KMerchantCard extends StatelessWidget {
  const KMerchantCard({
    super.key,
    required this.name,
    required this.subtitle,
    this.imageUrl,
    this.rating,
    this.chips = const [],
    this.dimmed = false,
    this.width = 200,
    this.onTap,
  });

  final String name;
  final String subtitle;
  final String? imageUrl;
  final String? rating;
  final List<Widget> chips;
  final bool dimmed;
  final double width;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => SizedBox(
    width: width,
    child: Material(
      color: K.surface,
      borderRadius: BorderRadius.circular(20),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(20),
        child: Ink(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(20),
            border: Border.all(color: K.line),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              ClipRRect(
                borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
                child: Stack(
                  children: [
                    Opacity(
                      opacity: dimmed ? 0.55 : 1,
                      child: KNetImage(
                        url: imageUrl,
                        height: 118,
                        width: width,
                        radius: 0,
                        fallbackIcon: Icons.storefront_outlined,
                      ),
                    ),
                    if (rating != null)
                      Positioned(
                        left: 10,
                        top: 10,
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
                          decoration: BoxDecoration(
                            color: K.bg.withValues(alpha: 0.78),
                            borderRadius: BorderRadius.circular(K.rFull),
                            border: Border.all(color: K.line2),
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              const Icon(Icons.star_rounded, size: 13, color: K.brand500),
                              const SizedBox(width: 3),
                              Text(
                                rating!,
                                style: const TextStyle(
                                  fontSize: 11.5,
                                  fontWeight: FontWeight.w700,
                                  color: K.text,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                  ],
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 14.5,
                        fontWeight: FontWeight.w700,
                        color: dimmed ? K.muted : K.text,
                      ),
                    ),
                    const SizedBox(height: 3),
                    Text(
                      subtitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 12, color: K.muted),
                    ),
                    if (chips.isNotEmpty) ...[
                      const SizedBox(height: 8),
                      Wrap(spacing: 6, runSpacing: 6, children: chips),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    ),
  );
}

/// Chip i vogël meta (koha, tarifa); neon kur do të theksohet (p.sh. dërgesë falas).
class KChip extends StatelessWidget {
  const KChip(this.label, {super.key, this.neon = false});

  final String label;
  final bool neon;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
    decoration: BoxDecoration(
      color: neon ? K.brand500.withValues(alpha: 0.10) : K.surface3,
      borderRadius: BorderRadius.circular(K.rFull),
    ),
    child: Text(
      label,
      style: TextStyle(
        fontSize: 11,
        fontWeight: FontWeight.w600,
        color: neon ? K.brand500 : K.textDim,
      ),
    ),
  );
}

/// Shifër e ditës për shoferin (fitimet, udhëtimet, vlerësimi).
class KKpi extends StatelessWidget {
  const KKpi({super.key, required this.label, required this.value, this.accent});

  final String label;
  final String value;
  final String? accent;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(12),
    decoration: BoxDecoration(
      color: K.surface,
      borderRadius: BorderRadius.circular(18),
      border: Border.all(color: K.line),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: const TextStyle(fontSize: 11.5, color: K.muted)),
        const SizedBox(height: 4),
        Text.rich(
          TextSpan(
            text: value,
            style: const TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w800,
              letterSpacing: -0.4,
              color: K.text,
              fontFeatures: [FontFeature.tabularFigures()],
            ),
            children: [
              if (accent != null)
                TextSpan(
                  text: ' $accent',
                  style: const TextStyle(fontSize: 12, color: K.brand500, letterSpacing: 0),
                ),
            ],
          ),
        ),
      ],
    ),
  );
}

/// Banderola e gjendjes aktive sipas mockup-it: jeshile e errët me kufi neon dhe shkëlqim.
class KNeonBanner extends StatelessWidget {
  const KNeonBanner({
    super.key,
    required this.icon,
    required this.title,
    required this.subtitle,
    this.onTap,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => Material(
    color: Colors.transparent,
    child: InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(18),
      child: Ink(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        decoration: BoxDecoration(
          gradient: const LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Color(0xFF132F0D), Color(0xFF0F2409)],
          ),
          borderRadius: BorderRadius.circular(18),
          border: Border.all(color: K.brand500.withValues(alpha: 0.45)),
          boxShadow: [BoxShadow(color: K.brand500.withValues(alpha: 0.12), blurRadius: 30)],
        ),
        child: Row(
          children: [
            Container(
              width: 38,
              height: 38,
              decoration: BoxDecoration(color: K.brand500, borderRadius: BorderRadius.circular(12)),
              child: Icon(icon, color: K.onBrand, size: 21),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: const TextStyle(
                      fontSize: 14.5,
                      fontWeight: FontWeight.w700,
                      color: K.text,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(subtitle, style: const TextStyle(fontSize: 12, color: K.accent2)),
                ],
              ),
            ),
            if (onTap != null) const Icon(Icons.chevron_right, color: K.brand500),
          ],
        ),
      ),
    ),
  );
}

/// Fusha e kërkimit e ballinës (vetëm pamje; prekja hap ekranin e kërkimit).
class KSearchBar extends StatelessWidget {
  const KSearchBar({super.key, required this.hint, this.onTap});

  final String hint;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => Material(
    color: K.surface,
    borderRadius: BorderRadius.circular(16),
    child: InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: Ink(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: K.line),
        ),
        child: Row(
          children: [
            const Icon(Icons.search, size: 20, color: K.muted),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                hint,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 15, color: K.muted),
              ),
            ),
          ],
        ),
      ),
    ),
  );
}
