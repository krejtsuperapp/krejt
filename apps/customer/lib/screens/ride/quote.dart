import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import 'destination.dart';
import 'tracking.dart';

String rideCategoryKey(RideCategory c) => 'ride.category.${c.name}';

/// Oferta e çmimit vjen nga serveri dhe vlen dy minuta (§18). Kur skadon, klienti
/// nuk e dërgon dot kërkesën — kërkon një çmim të ri, që askush të mos udhëtojë me çmim të vjetër.
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
  String _paymentMethod = 'cash';
  ApiError? _error;
  bool _loading = true;
  bool _requesting = false;
  Timer? _ticker;
  int _secondsLeft = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _quote());
  }

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
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
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('ride.quote.title'))),
      body: SafeArea(child: _body(context, locale)),
    );
  }

  Widget _body(BuildContext context, String locale) {
    if (_loading) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 88));
    }
    final result = _result;
    if (result == null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.tError(_error?.messageKey ?? 'errors.internal'),
          retryLabel: context.t('common.retry'),
          onRetry: _quote,
        ),
      );
    }
    if (result.quotes.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KEmpty(
          title: context.t('errors.outside_area'),
          icon: Icons.map_outlined,
          action: context.t('common.retry'),
          onAction: _quote,
        ),
      );
    }

    final quote = _selected;
    final expired = quote != null && _secondsLeft == 0;

    return Column(
      children: [
        Expanded(
          child: ListView(
            padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s4),
            children: [
              KCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    KRow(context.t('ride.pickup'), widget.pickup.label),
                    KRow(context.t('ride.dropoff'), widget.dropoff.label),
                    KRow(
                      context.t('ride.summary'),
                      context.t('ride.trip', {
                        'distance': formatDistance(result.distanceM, locale: locale),
                        'duration': formatDuration(result.durationS),
                      }),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: K.s5),
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
              const SizedBox(height: K.s4),
              KSectionHeader(context.t('ride.payment')),
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
                context.t('ride.price.fixed'),
                style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(K.s5, 0, K.s5, K.s5),
          child: Column(
            children: [
              Text(
                expired
                    ? context.t('ride.quote.expired')
                    : context.t('ride.quote.expires', {'s': '$_secondsLeft'}),
                style: TextStyle(fontSize: 13, color: expired ? K.warn : K.muted),
              ),
              const SizedBox(height: K.s2),
              if (expired)
                KButton(label: context.t('ride.quote.refresh'), onPressed: _quote)
              else
                KButton(
                  label: context.t('ride.request'),
                  busy: _requesting,
                  onPressed: _requesting ? null : _request,
                ),
            ],
          ),
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

  @override
  Widget build(BuildContext context) {
    final eta = quote.pickupEtaS;
    return KCard(
      onTap: onTap,
      highlight: selected,
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      context.t(rideCategoryKey(quote.category)),
                      style: const TextStyle(
                        fontSize: 16,
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
          KMoney(quote.priceMinor, currency: quote.currency, locale: locale, size: 20),
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
