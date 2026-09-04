import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart' hide Place;
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import 'destination.dart';
import 'map_scaffold.dart';
import 'tracking.dart';

String rideCategoryKey(RideCategory c) => 'ride.category.${c.name}';

/// Oferta e çmimit vjen nga serveri dhe vlen dy minuta (§18). Kur skadon, klienti
/// nuk e dërgon dot kërkesën — kërkon një çmim të ri, që askush të mos udhëtojë me çmim të vjetër.
/// Harta sipër tregon rrugën e vërtetë (gjeometria nga serveri); çmimi nuk varet prej saj.
class QuoteScreen extends StatefulWidget {
  const QuoteScreen({super.key, required this.pickup, required this.dropoff});

  final Place pickup;
  final Place dropoff;

  @override
  State<QuoteScreen> createState() => _QuoteScreenState();
}

class _QuoteScreenState extends State<QuoteScreen> {
  QuoteResult? _result;
  RideQuote? _selected;
  List<MapPoint>? _path;
  String _paymentMethod = 'cash';
  ApiError? _error;
  bool _loading = true;
  bool _requesting = false;
  Timer? _ticker;
  int _secondsLeft = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _quote();
      _route();
    });
  }

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  Future<void> _route() async {
    try {
      final r = await context.read<AppState>().api.routePath(
        widget.pickup.point,
        widget.dropoff.point,
      );
      if (!mounted) return;
      setState(() => _path = [for (final p in r.points) MapPoint(p.lat, p.lng)]);
    } on ApiError {
      // Pa gjeometri harta tregon vetëm dy pikat; oferta nuk varet prej saj.
    }
  }

  Future<void> _quote() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final result = await context.read<AppState>().api.quoteRide(
        pickup: widget.pickup.point,
        dropoff: widget.dropoff.point,
        pickupAddress: widget.pickup.label,
        dropoffAddress: widget.dropoff.label,
      );
      if (!mounted) return;
      setState(() {
        _result = result;
        _selected = result.quotes.isEmpty ? null : result.quotes.first;
        _loading = false;
      });
      _startTicker();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  void _startTicker() {
    _ticker?.cancel();
    void tick() {
      final quote = _selected;
      if (!mounted || quote == null) return;
      final left = quote.expiresAt.difference(DateTime.now()).inSeconds;
      setState(() => _secondsLeft = left < 0 ? 0 : left);
      if (left <= 0) _ticker?.cancel();
    }

    tick();
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) => tick());
  }

  Future<void> _request() async {
    final quote = _selected;
    if (quote == null || quote.expired) return;
    setState(() => _requesting = true);
    final state = context.read<AppState>();
    try {
      final ride = await state.api.requestRide(quoteId: quote.id, paymentMethod: _paymentMethod);
      if (!mounted) return;
      await Navigator.of(
        context,
      ).pushReplacement(MaterialPageRoute<void>(builder: (_) => TrackingScreen(rideId: ride.id)));
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _requesting = false);
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    }
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    return MapScaffold.panel(
      title: context.t('ride.quote.title'),
      markers: [
        markerOf(widget.pickup.point.lat, widget.pickup.point.lng, MapMarkerKind.pickup),
        markerOf(widget.dropoff.point.lat, widget.dropoff.point.lng, MapMarkerKind.dropoff),
      ],
      path: _path,
      panel: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const Center(child: KSheetHandle()),
            ConstrainedBox(
              constraints: BoxConstraints(maxHeight: MediaQuery.sizeOf(context).height * 0.6),
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, K.s4),
                child: _body(context, locale),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _body(BuildContext context, String locale) {
    if (_loading) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          RouteEnds(pickup: widget.pickup.label, dropoff: widget.dropoff.label),
          const SizedBox(height: K.s4),
          const KSkeleton(height: 72, count: 3),
        ],
      );
    }
    final result = _result;
    if (result == null) {
      return KError(
        message: context.tError(_error?.messageKey ?? 'errors.internal'),
        retryLabel: context.t('common.retry'),
        onRetry: _quote,
      );
    }
    if (result.quotes.isEmpty) {
      return KEmpty(
        title: context.t('errors.outside_area'),
        icon: Icons.map_outlined,
        action: context.t('common.retry'),
        onAction: _quote,
      );
    }

    final quote = _selected;
    final expired = quote != null && _secondsLeft == 0;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: RouteEnds(pickup: widget.pickup.label, dropoff: widget.dropoff.label),
            ),
            const SizedBox(width: K.s3),
            KChip(
              context.t('ride.trip', {
                'distance': formatDistance(result.distanceM, locale: locale),
                'duration': formatDuration(result.durationS),
              }),
            ),
          ],
        ),
        const SizedBox(height: K.s4),
        for (final q in result.quotes)
          Padding(
            padding: const EdgeInsets.only(bottom: K.s2),
            child: _CategoryRow(
              quote: q,
              locale: locale,
              selected: _selected?.id == q.id,
              onTap: () {
                setState(() => _selected = q);
                _startTicker();
              },
            ),
          ),
        const SizedBox(height: K.s3),
        Row(
          children: [
            Expanded(
              child: _PaymentChoice(
                icon: Icons.payments_outlined,
                label: context.t('ride.payment.cash'),
                selected: _paymentMethod == 'cash',
                onTap: () => setState(() => _paymentMethod = 'cash'),
              ),
            ),
            const SizedBox(width: K.s3),
            Expanded(
              child: _PaymentChoice(
                icon: Icons.account_balance_wallet_outlined,
                label: context.t('ride.payment.wallet'),
                selected: _paymentMethod == 'wallet',
                onTap: () => setState(() => _paymentMethod = 'wallet'),
              ),
            ),
          ],
        ),
        const SizedBox(height: K.s4),
        Text(
          expired
              ? context.t('ride.quote.expired')
              : '${context.t('ride.quote.expires', {'s': '$_secondsLeft'})} · '
                    '${context.t('ride.price.fixed')}',
          textAlign: TextAlign.center,
          style: TextStyle(fontSize: 12, color: expired ? K.warn : K.muted, height: 1.4),
        ),
        const SizedBox(height: K.s2),
        if (expired)
          KButton(label: context.t('ride.quote.refresh'), icon: Icons.refresh, onPressed: _quote)
        else
          KButton(
            label: quote == null
                ? context.t('ride.request')
                : '${context.t('ride.request')} · '
                      '${formatMinor(quote.priceMinor, currency: quote.currency, locale: locale)}',
            busy: _requesting,
            onPressed: _requesting ? null : _request,
          ),
      ],
    );
  }
}

class _CategoryRow extends StatelessWidget {
  const _CategoryRow({
    required this.quote,
    required this.locale,
    required this.selected,
    required this.onTap,
  });

  final RideQuote quote;
  final String locale;
  final bool selected;
  final VoidCallback onTap;

  static IconData _icon(RideCategory c) {
    switch (c.name) {
      case 'comfort':
      case 'premium':
        return Icons.airline_seat_recline_extra;
      case 'xl':
      case 'van':
        return Icons.airport_shuttle_outlined;
      case 'taxi':
        return Icons.local_taxi_outlined;
    }
    return Icons.directions_car_outlined;
  }

  @override
  Widget build(BuildContext context) {
    final eta = quote.pickupEtaS;
    return KCard(
      onTap: onTap,
      highlight: selected,
      padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
      child: Row(
        children: [
          Container(
            width: 42,
            height: 42,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: selected ? K.brand500.withValues(alpha: 0.14) : K.surface2,
              borderRadius: BorderRadius.circular(K.rSm),
            ),
            child: Icon(_icon(quote.category), size: 22, color: selected ? K.brand400 : K.textDim),
          ),
          const SizedBox(width: K.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      context.t(rideCategoryKey(quote.category)),
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                        color: K.text,
                      ),
                    ),
                    const SizedBox(width: K.s2),
                    if (quote.surging) KBadge(context.t('ride.quote.surge'), tone: KTone.warn),
                  ],
                ),
                const SizedBox(height: 2),
                Text(
                  [
                    context.t('ride.seats', {'n': '${quote.seats}'}),
                    if (eta != null) context.t('ride.eta', {'min': '${(eta / 60).ceil()}'}),
                  ].join(' · '),
                  style: const TextStyle(fontSize: 12, color: K.muted),
                ),
              ],
            ),
          ),
          KMoney(quote.priceMinor, currency: quote.currency, locale: locale, size: 18),
        ],
      ),
    );
  }
}

class _PaymentChoice extends StatelessWidget {
  const _PaymentChoice({
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
