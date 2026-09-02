import 'package:flutter/material.dart';

import '../tokens.dart';
import 'buttons.dart';

/// Gjendjet e detyrueshme të çdo ekrani (§55): loading, empty, error, offline, forbidden, maintenance.
/// Asnjë ekran nuk lejohet të tregojë vetëm një spinner pa kontekst.
class KLoading extends StatelessWidget {
  const KLoading({super.key, this.label});

  final String? label;

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        const SizedBox(width: 30, height: 30, child: CircularProgressIndicator(strokeWidth: 2.6)),
        if (label != null) ...[
          const SizedBox(height: K.s4),
          Text(label!, style: const TextStyle(color: K.muted)),
        ],
      ],
    ),
  );
}

/// Skelet për listat — më i qetë se spinner-i kur dimë formën e përmbajtjes (§61).
class KSkeleton extends StatefulWidget {
  const KSkeleton({super.key, this.height = 76, this.count = 3});

  final double height;
  final int count;

  @override
  State<KSkeleton> createState() => _KSkeletonState();
}

class _KSkeletonState extends State<KSkeleton> with SingleTickerProviderStateMixin {
  late final AnimationController _c = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1100),
  )..repeat(reverse: true);

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Column(
    children: List.generate(
      widget.count,
      (i) => Padding(
        padding: const EdgeInsets.only(bottom: K.s3),
        child: FadeTransition(
          opacity: Tween<double>(begin: 0.45, end: 0.85).animate(_c),
          child: Container(
            height: widget.height,
            decoration: BoxDecoration(
              color: K.surface2,
              borderRadius: BorderRadius.circular(K.rMd),
              border: Border.all(color: K.line),
            ),
          ),
        ),
      ),
    ),
  );
}

/// Gjendje e zbrazët me veprim — kurrë vetëm "nuk ka të dhëna".
class KEmpty extends StatelessWidget {
  const KEmpty({
    super.key,
    required this.title,
    this.message,
    this.icon = Icons.inbox_outlined,
    this.action,
    this.onAction,
  });

  final String title;
  final String? message;
  final IconData icon;
  final String? action;
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(K.s6),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 64,
            height: 64,
            decoration: BoxDecoration(
              color: K.surface2,
              borderRadius: BorderRadius.circular(K.rLg),
            ),
            child: Icon(icon, color: K.muted, size: 30),
          ),
          const SizedBox(height: K.s4),
          Text(
            title,
            textAlign: TextAlign.center,
            style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: K.text),
          ),
          if (message != null) ...[
            const SizedBox(height: K.s2),
            Text(
              message!,
              textAlign: TextAlign.center,
              style: const TextStyle(color: K.muted, height: 1.45),
            ),
          ],
          if (action != null) ...[
            const SizedBox(height: K.s5),
            KButton(label: action!, onPressed: onAction, expanded: false),
          ],
        ],
      ),
    ),
  );
}

/// Gabim me shkak dhe rrugëdalje: çdo gabim tregon çfarë ndodhi dhe si vazhdohet (§55, §57).
class KError extends StatelessWidget {
  const KError({
    super.key,
    required this.message,
    this.title,
    this.onRetry,
    this.retryLabel,
    this.icon = Icons.error_outline,
  });

  final String message;
  final String? title;
  final VoidCallback? onRetry;
  final String? retryLabel;
  final IconData icon;

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(K.s6),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 64,
            height: 64,
            decoration: BoxDecoration(
              color: K.dangerBg,
              borderRadius: BorderRadius.circular(K.rLg),
            ),
            child: Icon(icon, color: K.danger, size: 30),
          ),
          const SizedBox(height: K.s4),
          Text(
            title ?? 'Diçka nuk shkoi',
            textAlign: TextAlign.center,
            style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: K.text),
          ),
          const SizedBox(height: K.s2),
          Text(
            message,
            textAlign: TextAlign.center,
            style: const TextStyle(color: K.muted, height: 1.45),
          ),
          if (onRetry != null) ...[
            const SizedBox(height: K.s5),
            KButton(
              label: retryLabel ?? 'Provo sërish',
              onPressed: onRetry,
              icon: Icons.refresh,
              expanded: false,
            ),
          ],
        ],
      ),
    ),
  );
}

/// Shirit i vazhdueshëm kur pajisja është offline (§62).
class KOfflineBar extends StatelessWidget {
  const KOfflineBar({super.key, required this.label});

  final String label;

  @override
  Widget build(BuildContext context) => Container(
    width: double.infinity,
    color: K.warnBg,
    padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s2),
    child: Row(
      children: [
        const Icon(Icons.wifi_off_rounded, size: 16, color: K.warn),
        const SizedBox(width: K.s2),
        Expanded(
          child: Text(
            label,
            style: const TextStyle(color: K.warn, fontSize: 13, fontWeight: FontWeight.w600),
          ),
        ),
      ],
    ),
  );
}
