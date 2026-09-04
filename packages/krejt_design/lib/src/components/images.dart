import 'package:flutter/material.dart';

import '../tokens.dart';

/// Imazh nga rrjeti me vend të rezervuar dhe rënie të qetë: pa URL ose me gabim shfaq ikonën,
/// kurrë një katror bosh apo një gabim të kuq. Përmasat i vendos prindi.
class KNetImage extends StatelessWidget {
  const KNetImage({
    super.key,
    required this.url,
    this.width,
    this.height,
    this.radius = K.rSm,
    this.fit = BoxFit.cover,
    this.fallbackIcon = Icons.image_outlined,
  });

  final String? url;
  final double? width;
  final double? height;
  final double radius;
  final BoxFit fit;
  final IconData fallbackIcon;

  @override
  Widget build(BuildContext context) {
    final placeholder = Container(
      width: width,
      height: height,
      color: K.surface2,
      alignment: Alignment.center,
      child: Icon(fallbackIcon, color: K.muted, size: 22),
    );
    final u = url;
    return ClipRRect(
      borderRadius: BorderRadius.circular(radius),
      child: u == null || u.isEmpty
          ? placeholder
          : Image.network(
              u,
              width: width,
              height: height,
              fit: fit,
              gaplessPlayback: true,
              errorBuilder: (_, _, _) => placeholder,
              loadingBuilder: (_, child, progress) => progress == null ? child : placeholder,
            ),
    );
  }
}

/// Avatar rrethor: fotoja e profilit kur ka, ndryshe inicialet mbi sfond të markës.
class KAvatar extends StatelessWidget {
  const KAvatar({super.key, this.url, required this.initials, this.size = 48});

  final String? url;
  final String initials;
  final double size;

  @override
  Widget build(BuildContext context) {
    final u = url;
    final fallback = Container(
      width: size,
      height: size,
      alignment: Alignment.center,
      color: K.brand500.withValues(alpha: 0.18),
      child: Text(
        initials,
        style: TextStyle(
          fontSize: size * 0.38,
          fontWeight: FontWeight.w700,
          color: K.brand400,
          letterSpacing: 0.5,
        ),
      ),
    );
    return ClipOval(
      child: u == null || u.isEmpty
          ? fallback
          : Image.network(
              u,
              width: size,
              height: size,
              fit: BoxFit.cover,
              gaplessPlayback: true,
              errorBuilder: (_, _, _) => fallback,
              loadingBuilder: (_, child, progress) => progress == null ? child : fallback,
            ),
    );
  }
}
