import 'package:flutter/material.dart';

import '../tokens.dart';

/// Butoni primar me gradientin e markës. Gjithmonë ≥ 48 px lartësi (§56) dhe me gjendje "duke punuar" (§55).
class KButton extends StatelessWidget {
  const KButton({
    super.key,
    required this.label,
    this.onPressed,
    this.busy = false,
    this.icon,
    this.expanded = true,
    this.danger = false,
  });

  final String label;
  final VoidCallback? onPressed;
  final bool busy;
  final IconData? icon;
  final bool expanded;
  final bool danger;

  @override
  Widget build(BuildContext context) {
    final enabled = onPressed != null && !busy;
    final child = Row(
      mainAxisSize: expanded ? MainAxisSize.max : MainAxisSize.min,
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        if (busy)
          const SizedBox(
            width: 18,
            height: 18,
            child: CircularProgressIndicator(strokeWidth: 2.2, color: K.onBrand),
          )
        else if (icon != null)
          Icon(icon, size: 20, color: K.onBrand),
        if (busy || icon != null) const SizedBox(width: K.s2),
        Flexible(
          child: Text(
            label,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(color: K.onBrand, fontWeight: FontWeight.w700, fontSize: 15.5),
          ),
        ),
      ],
    );

    return Opacity(
      opacity: enabled ? 1 : 0.55,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: enabled ? onPressed : null,
          borderRadius: BorderRadius.circular(K.rMd),
          child: Ink(
            height: K.minTap,
            decoration: BoxDecoration(
              gradient: danger ? null : K.gradient,
              color: danger ? K.danger : null,
              borderRadius: BorderRadius.circular(K.rMd),
            ),
            padding: const EdgeInsets.symmetric(horizontal: K.s5),
            child: Center(child: child),
          ),
        ),
      ),
    );
  }
}

/// Butoni dytësor: kontur, pa mbushje — për veprime jo-primare (§7 hierarkia).
class KOutlineButton extends StatelessWidget {
  const KOutlineButton({
    super.key,
    required this.label,
    this.onPressed,
    this.icon,
    this.danger = false,
  });

  final String label;
  final VoidCallback? onPressed;
  final IconData? icon;
  final bool danger;

  @override
  Widget build(BuildContext context) {
    final c = danger ? K.danger : K.text;
    return OutlinedButton.icon(
      onPressed: onPressed,
      icon: icon == null ? const SizedBox.shrink() : Icon(icon, size: 19, color: c),
      label: Text(
        label,
        style: TextStyle(color: c, fontWeight: FontWeight.w600, fontSize: 15),
      ),
      style: OutlinedButton.styleFrom(
        minimumSize: const Size.fromHeight(K.minTap),
        side: BorderSide(color: danger ? K.danger : K.line2),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(K.rMd)),
      ),
    );
  }
}

/// Lidhje tekstuale (p.sh. "Shiko të gjitha").
class KTextLink extends StatelessWidget {
  const KTextLink({super.key, required this.label, this.onPressed});

  final String label;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) => TextButton(
    onPressed: onPressed,
    style: TextButton.styleFrom(
      padding: const EdgeInsets.symmetric(horizontal: K.s2, vertical: K.s1),
      minimumSize: Size.zero,
      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
    ),
    child: Text(
      label,
      style: const TextStyle(color: K.brand400, fontWeight: FontWeight.w600, fontSize: 14),
    ),
  );
}
