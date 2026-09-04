import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../coupon_field.dart';
import '../active_banner.dart';
import '../ride/map_scaffold.dart';
import '../ride/place_search.dart';
import 'parcel_tracking.dart';

/// Dërgimi i një pakoje: madhësia, marrja dhe dorëzimi (me koordinata të vërteta), marrësi,
/// pastaj çmimi nga serveri (2 min) dhe dërgimi. Harta sipër tregon rrugën sapo dihen dy pikat.
class NewParcelScreen extends StatefulWidget {
  const NewParcelScreen({super.key});

  @override
  State<NewParcelScreen> createState() => _NewParcelScreenState();
}

class _NewParcelScreenState extends State<NewParcelScreen> {
  final _senderName = TextEditingController();
  final _senderPhone = TextEditingController();
  final _recipientName = TextEditingController();
  final _recipientPhone = TextEditingController();
  final _note = TextEditingController();

  String _size = 's';
  PickedPlace? _pickup;
  PickedPlace? _dropoff;
  List<MapPoint>? _path;
  ParcelQuote? _quote;
  String _paymentMethod = 'cash';
  CouponApplied? _coupon;
  String? _recipientError;
  bool _quoting = false;
  bool _sending = false;
  Timer? _ticker;
  int _secondsLeft = 0;

  @override
  void dispose() {
    _ticker?.cancel();
    for (final c in [_senderName, _senderPhone, _recipientName, _recipientPhone, _note]) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _pick(bool pickup) async {
    final place = await showPlaceSearch(
      context,
      title: context.t(pickup ? 'parcel.pickup.hint' : 'parcel.dropoff.hint'),
    );
    if (place == null || !mounted) return;
    setState(() {
      if (pickup) {
        _pickup = place;
      } else {
        _dropoff = place;
      }
      _quote = null;
      _path = null;
    });
    unawaited(_route());
  }

  Future<void> _route() async {
    final a = _pickup, b = _dropoff;
    if (a == null || b == null) return;
    try {
      final r = await context.read<AppState>().api.routePath(a.point, b.point);
      if (!mounted || _pickup != a || _dropoff != b) return;
      setState(() => _path = [for (final p in r.points) MapPoint(p.lat, p.lng)]);
    } on ApiError {
      // Pa gjeometri harta tregon vetëm dy pikat.
    }
  }

  String get _e164 {
    final digits = _recipientPhone.text.replaceAll(RegExp(r'\D'), '');
    if (digits.startsWith('383') || digits.length > 10) return '+$digits';
    if (digits.startsWith('0')) return '+383${digits.substring(1)}';
    return '+383$digits';
  }

  Future<void> _getQuote() async {
    final a = _pickup, b = _dropoff;
    if (a == null || b == null) return;
    setState(() => _quoting = true);
    try {
      final q = await context.read<AppState>().api.quoteParcel(
        size: _size,
        pickup: a.point,
        dropoff: b.point,
        pickupAddress: a.label,
        dropoffAddress: b.label,
      );
      if (!mounted) return;
      setState(() => _quote = q);
      _startTicker(q);
    } on ApiError catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    } finally {
      if (mounted) setState(() => _quoting = false);
    }
  }

  void _startTicker(ParcelQuote q) {
    _ticker?.cancel();
    void tick() {
      if (!mounted) return;
      final left = q.expiresAt.difference(DateTime.now()).inSeconds;
      setState(() => _secondsLeft = left < 0 ? 0 : left);
      if (left <= 0) _ticker?.cancel();
    }

    tick();
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) => tick());
  }

  Future<void> _send() async {
    final q = _quote;
    if (q == null || q.expired) return;
    final name = _recipientName.text.trim();
    final digits = _recipientPhone.text.replaceAll(RegExp(r'\D'), '');
    if (name.isEmpty || digits.length < 8) {
      setState(() => _recipientError = context.t('errors.validation'));
      return;
    }
    setState(() {
      _sending = true;
      _recipientError = null;
    });
    final state = context.read<AppState>();
    try {
      final parcel = await state.api.createParcel(
        quoteId: q.id,
        paymentMethod: _paymentMethod,
        recipientName: name,
        recipientPhone: _e164,
        couponCode: _coupon?.code,
        pickupContactName: _senderName.text.trim().isEmpty ? null : _senderName.text.trim(),
        pickupContactPhone: _senderPhone.text.trim().isEmpty ? null : _senderPhone.text.trim(),
        note: _note.text.trim().isEmpty ? null : _note.text.trim(),
      );
      if (!mounted) return;
      unawaited(state.refreshHome());
      await Navigator.of(context).pushReplacement(
        MaterialPageRoute<void>(builder: (_) => ParcelTrackingScreen(parcelId: parcel.id)),
      );
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _sending = false);
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    }
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    final quote = _quote;
    final expired = quote != null && _secondsLeft == 0;
    return MapScaffold.panel(
      title: context.t('parcel.new'),
      markers: [
        if (_pickup != null) markerOf(_pickup!.point.lat, _pickup!.point.lng, MapMarkerKind.pickup),
        if (_dropoff != null)
          markerOf(_dropoff!.point.lat, _dropoff!.point.lng, MapMarkerKind.dropoff),
      ],
      path: _path,
      panel: Padding(
        padding: EdgeInsets.only(bottom: MediaQuery.viewInsetsOf(context).bottom),
        child: SafeArea(
          top: false,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Center(child: KSheetHandle()),
              ConstrainedBox(
                constraints: BoxConstraints(maxHeight: MediaQuery.sizeOf(context).height * 0.62),
                child: SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, K.s4),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const ActiveBanner(kind: ActiveKind.parcel),
                      _PlaceField(
                        color: K.brand500,
                        square: false,
                        hint: context.t('parcel.pickup.hint'),
                        value: _pickup?.label,
                        onTap: () => _pick(true),
                      ),
                      const SizedBox(height: K.s2),
                      _PlaceField(
                        color: K.info,
                        square: true,
                        hint: context.t('parcel.dropoff.hint'),
                        value: _dropoff?.label,
                        onTap: () => _pick(false),
                      ),
                      const SizedBox(height: K.s4),
                      KSectionHeader(context.t('parcel.size')),
                      const SizedBox(height: K.s2),
                      Row(
                        children: [
                          for (final s in const ['s', 'm', 'l']) ...[
                            Expanded(
                              child: _SizeChip(
                                size: s,
                                selected: _size == s,
                                onTap: () => setState(() {
                                  _size = s;
                                  _quote = null;
                                }),
                              ),
                            ),
                            if (s != 'l') const SizedBox(width: K.s2),
                          ],
                        ],
                      ),
                      const SizedBox(height: K.s4),
                      KSectionHeader(context.t('parcel.recipient')),
                      const SizedBox(height: K.s2),
                      KField(
                        label: context.t('parcel.recipient.name'),
                        controller: _recipientName,
                        error: _recipientError,
                        textInputAction: TextInputAction.next,
                        onChanged: (_) => setState(() => _recipientError = null),
                      ),
                      const SizedBox(height: K.s3),
                      KField(
                        label: context.t('parcel.recipient.phone'),
                        controller: _recipientPhone,
                        hint: context.t('parcel.recipient.phone.hint'),
                        keyboardType: TextInputType.phone,
                        inputFormatters: [FilteringTextInputFormatter.allow(RegExp(r'[0-9+ ]'))],
                        textInputAction: TextInputAction.next,
                        onChanged: (_) => setState(() => _recipientError = null),
                      ),
                      const SizedBox(height: K.s3),
                      KField(
                        label: context.t('parcel.note'),
                        controller: _note,
                        maxLines: 2,
                        maxLength: 300,
                      ),
                      const SizedBox(height: K.s4),
                      KSectionHeader(context.t('ride.payment')),
                      const SizedBox(height: K.s2),
                      Row(
                        children: [
                          Expanded(
                            child: _Choice(
                              icon: Icons.payments_outlined,
                              label: context.t('ride.payment.cash'),
                              selected: _paymentMethod == 'cash',
                              onTap: () => setState(() => _paymentMethod = 'cash'),
                            ),
                          ),
                          const SizedBox(width: K.s3),
                          Expanded(
                            child: _Choice(
                              icon: Icons.account_balance_wallet_outlined,
                              label: context.t('ride.payment.wallet'),
                              selected: _paymentMethod == 'wallet',
                              onTap: () => setState(() => _paymentMethod = 'wallet'),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: K.s4),
                      if (quote != null)
                        CouponField(
                          scope: 'parcels',
                          amountMinor: quote.priceMinor,
                          applied: _coupon,
                          onChanged: (c) => setState(() => _coupon = c),
                        ),
                      if (quote != null) const SizedBox(height: K.s4),
                      if (quote != null && !expired) ...[
                        KCard(
                          highlight: true,
                          child: Row(
                            children: [
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      context.t('ride.trip', {
                                        'distance': formatDistance(quote.distanceM, locale: locale),
                                        'duration': formatDuration(quote.durationS),
                                      }),
                                      style: const TextStyle(
                                        fontSize: 14,
                                        fontWeight: FontWeight.w600,
                                        color: K.text,
                                      ),
                                    ),
                                    const SizedBox(height: 2),
                                    Text(
                                      context.t('ride.quote.expires', {'s': '$_secondsLeft'}),
                                      style: const TextStyle(fontSize: 12, color: K.muted),
                                    ),
                                  ],
                                ),
                              ),
                              KMoney(
                                quote.priceMinor,
                                currency: quote.currency,
                                locale: locale,
                                size: 24,
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(height: K.s2),
                        Text(
                          context.t('parcel.price.fixed'),
                          style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
                        ),
                        const SizedBox(height: K.s3),
                        KButton(
                          label:
                              '${context.t('parcel.send')} · '
                              '${formatMinor(quote.priceMinor - (_coupon?.discountMinor ?? 0), currency: quote.currency, locale: locale)}',
                          icon: Icons.send_outlined,
                          busy: _sending,
                          onPressed: _sending ? null : _send,
                        ),
                      ] else
                        KButton(
                          label: context.t(expired ? 'ride.quote.refresh' : 'parcel.quote'),
                          icon: Icons.receipt_long_outlined,
                          busy: _quoting,
                          onPressed: (_pickup != null && _dropoff != null && !_quoting)
                              ? _getQuote
                              : null,
                        ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _PlaceField extends StatelessWidget {
  const _PlaceField({
    required this.color,
    required this.square,
    required this.hint,
    required this.value,
    required this.onTap,
  });

  final Color color;
  final bool square;
  final String hint;
  final String? value;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => InkWell(
    onTap: onTap,
    borderRadius: BorderRadius.circular(K.rMd),
    child: Container(
      height: 52,
      padding: const EdgeInsets.symmetric(horizontal: K.s4),
      decoration: BoxDecoration(
        color: K.surface2,
        borderRadius: BorderRadius.circular(K.rMd),
        border: Border.all(color: value == null ? K.line : color.withValues(alpha: 0.6)),
      ),
      child: Row(
        children: [
          Container(
            width: 10,
            height: 10,
            decoration: BoxDecoration(
              color: color,
              borderRadius: BorderRadius.circular(square ? 2 : K.rFull),
              boxShadow: [BoxShadow(color: color.withValues(alpha: 0.55), blurRadius: 8)],
            ),
          ),
          const SizedBox(width: K.s3),
          Expanded(
            child: Text(
              value ?? hint,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w600,
                color: value == null ? K.muted : K.text,
              ),
            ),
          ),
          Icon(value == null ? Icons.search : Icons.edit_outlined, size: 18, color: K.muted),
        ],
      ),
    ),
  );
}

class _SizeChip extends StatelessWidget {
  const _SizeChip({required this.size, required this.selected, required this.onTap});

  final String size;
  final bool selected;
  final VoidCallback onTap;

  static const _icons = {
    's': Icons.mail_outline,
    'm': Icons.inventory_2_outlined,
    'l': Icons.local_shipping_outlined,
  };

  @override
  Widget build(BuildContext context) => KCard(
    onTap: onTap,
    highlight: selected,
    padding: const EdgeInsets.symmetric(horizontal: K.s3, vertical: K.s3),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(_icons[size], size: 22, color: selected ? K.brand400 : K.muted),
        const SizedBox(height: K.s2),
        Text(
          context.t('parcel.size.$size'),
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w700,
            color: selected ? K.text : K.textDim,
          ),
        ),
        Text(
          context.t('parcel.size.$size.hint'),
          maxLines: 2,
          style: const TextStyle(fontSize: 11, color: K.muted, height: 1.3),
        ),
      ],
    ),
  );
}

class _Choice extends StatelessWidget {
  const _Choice({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => KCard(
    onTap: onTap,
    highlight: selected,
    padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
    child: SizedBox(
      height: K.minTap - K.s4,
      child: Row(
        children: [
          Icon(icon, size: 20, color: selected ? K.brand400 : K.muted),
          const SizedBox(width: K.s2),
          Expanded(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: selected ? K.text : K.textDim,
              ),
            ),
          ),
        ],
      ),
    ),
  );
}
