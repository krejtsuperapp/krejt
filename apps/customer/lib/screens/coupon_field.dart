import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Fusha e kodit të zbritjes. Kodi kontrollohet te serveri para checkout-it: zbritja që shfaqet
/// këtu është e njëjta që do të zbatohet, sepse e llogarit vetëm serveri (§35).
class CouponField extends StatefulWidget {
  const CouponField({
    super.key,
    required this.scope,
    required this.amountMinor,
    required this.onChanged,
    this.applied,
  });

  /// `food` ose `parcels`.
  final String scope;

  /// Baza mbi të cilën llogaritet zbritja (artikujt te ushqimi, çmimi te pakoja).
  final int amountMinor;

  /// Kuponi i aplikuar ose null; ekrani prind e mban dhe e dërgon te checkout-i.
  final CouponApplied? applied;
  final ValueChanged<CouponApplied?> onChanged;

  @override
  State<CouponField> createState() => _CouponFieldState();
}

class _CouponFieldState extends State<CouponField> {
  final _code = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  Future<void> _apply() async {
    final code = _code.text.trim();
    if (code.isEmpty || _busy) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final applied = await context.read<AppState>().api.checkCoupon(
        code: code,
        scope: widget.scope,
        amountMinor: widget.amountMinor,
      );
      if (!mounted) return;
      widget.onChanged(applied);
      setState(() => _busy = false);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = context.tError(e.messageKey);
        _busy = false;
      });
    }
  }

  void _remove() {
    _code.clear();
    setState(() => _error = null);
    widget.onChanged(null);
  }

  @override
  Widget build(BuildContext context) {
    final applied = widget.applied;
    final locale = context.watch<AppState>().locale;
    if (applied != null) {
      return KCard(
        highlight: true,
        padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
        child: Row(
          children: [
            const Icon(Icons.local_offer_outlined, size: 20, color: K.brand400),
            const SizedBox(width: K.s3),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    applied.code,
                    style: const TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w700,
                      color: K.text,
                      letterSpacing: 1,
                    ),
                  ),
                  Text(
                    '${context.t('coupon.applied')} · −${formatMinor(applied.discountMinor, locale: locale)}',
                    style: const TextStyle(fontSize: 12, color: K.muted),
                  ),
                ],
              ),
            ),
            KTextLink(label: context.t('coupon.remove'), onPressed: _remove),
          ],
        ),
      );
    }
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: KField(
            label: context.t('coupon.label'),
            controller: _code,
            hint: context.t('coupon.hint'),
            error: _error,
            maxLength: 32,
            textInputAction: TextInputAction.done,
            inputFormatters: [
              FilteringTextInputFormatter.allow(RegExp(r'[A-Za-z0-9\-]')),
              TextInputFormatter.withFunction(
                (_, next) => next.copyWith(text: next.text.toUpperCase()),
              ),
            ],
            onChanged: (_) => setState(() => _error = null),
            onSubmitted: (_) => _apply(),
          ),
        ),
        const SizedBox(width: K.s3),
        Padding(
          padding: const EdgeInsets.only(top: 22),
          child: KOutlineButton(label: context.t('coupon.apply'), onPressed: _busy ? null : _apply),
        ),
      ],
    );
  }
}
