import 'package:flutter/material.dart';

import '../tokens.dart';

/// Kartë e thjeshtë me kufi — baza e çdo seksioni.
class KCard extends StatelessWidget {
  const KCard({super.key, required this.child, this.padding, this.onTap, this.highlight = false});

  final Widget child;
  final EdgeInsetsGeometry? padding;
  final VoidCallback? onTap;
  final bool highlight;

  @override
  Widget build(BuildContext context) {
    final content = Padding(padding: padding ?? const EdgeInsets.all(K.s4), child: child);
    return Material(
      color: K.surface,
      borderRadius: BorderRadius.circular(K.rMd),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(K.rMd),
        child: Ink(
          decoration: BoxDecoration(
            color: highlight ? K.brand500.withValues(alpha: 0.10) : null,
            borderRadius: BorderRadius.circular(K.rMd),
            border: Border.all(color: highlight ? K.brand500.withValues(alpha: 0.55) : K.line),
          ),
          child: content,
        ),
      ),
    );
  }
}

/// Banderolë për diçka aktive (udhëtim ose porosi në ecje) — §60 trust UX.
class KActiveBanner extends StatelessWidget {
  const KActiveBanner({
    super.key,
    required this.icon,
    required this.title,
    required this.subtitle,
    this.progress,
    this.onTap,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final double? progress;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => KCard(
    highlight: true,
    onTap: onTap,
    child: Row(
      children: [
        Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            gradient: K.gradient,
            borderRadius: BorderRadius.circular(K.rSm),
          ),
          child: Icon(icon, color: K.onBrand, size: 21),
        ),
        const SizedBox(width: K.s3),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 15, color: K.text),
              ),
              const SizedBox(height: 2),
              Text(subtitle, style: const TextStyle(color: K.textDim, fontSize: 13)),
              if (progress != null) ...[
                const SizedBox(height: K.s2),
                ClipRRect(
                  borderRadius: BorderRadius.circular(K.rFull),
                  child: LinearProgressIndicator(value: progress, minHeight: 5),
                ),
              ],
            ],
          ),
        ),
        if (onTap != null) const Icon(Icons.chevron_right, color: K.muted),
      ],
    ),
  );
}

/// Rresht çelësi–vlerë për përmbledhje çmimesh dhe detajesh.
class KRow extends StatelessWidget {
  const KRow(this.label, this.value, {super.key, this.strong = false, this.valueColor});

  final String label;
  final String value;
  final bool strong;
  final Color? valueColor;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 7),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Text(
            label,
            style: TextStyle(
              color: strong ? K.text : K.muted,
              fontSize: 14,
              fontWeight: strong ? FontWeight.w700 : FontWeight.w400,
            ),
          ),
        ),
        const SizedBox(width: K.s3),
        Text(
          value,
          style: TextStyle(
            color: valueColor ?? K.text,
            fontSize: strong ? 16 : 14,
            fontWeight: strong ? FontWeight.w800 : FontWeight.w600,
          ),
        ),
      ],
    ),
  );
}

/// Etiketë statusi me semantikë (§55 gjendjet, §67 statuset).
class KBadge extends StatelessWidget {
  const KBadge(this.label, {super.key, this.tone = KTone.neutral});

  final String label;
  final KTone tone;

  @override
  Widget build(BuildContext context) {
    final (fg, bg) = switch (tone) {
      KTone.brand => (K.brand400, K.brand500.withValues(alpha: 0.16)),
      KTone.ok => (K.ok, K.okBg),
      KTone.warn => (K.warn, K.warnBg),
      KTone.danger => (K.danger, K.dangerBg),
      KTone.info => (K.info, K.infoBg),
      KTone.neutral => (K.textDim, K.surface3),
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(K.rFull)),
      child: Text(
        label,
        style: TextStyle(color: fg, fontSize: 12, fontWeight: FontWeight.w700),
      ),
    );
  }
}

enum KTone { neutral, brand, ok, warn, danger, info }

/// Titull seksioni me veprim opsional në të djathtë.
class KSectionHeader extends StatelessWidget {
  const KSectionHeader(this.title, {super.key, this.action, this.onAction});

  final String title;
  final String? action;
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: K.s3, top: K.s5),
    child: Row(
      children: [
        Expanded(
          child: Text(
            title,
            style: const TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w800,
              color: K.text,
              letterSpacing: -0.3,
            ),
          ),
        ),
        if (action != null)
          TextButton(
            onPressed: onAction,
            style: TextButton.styleFrom(padding: EdgeInsets.zero, minimumSize: Size.zero),
            child: Text(
              action!,
              style: const TextStyle(color: K.brand400, fontWeight: FontWeight.w600),
            ),
          ),
      ],
    ),
  );
}
