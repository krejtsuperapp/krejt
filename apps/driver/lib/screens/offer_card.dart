import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import '../state/work_state.dart';
import 'active_ride.dart';

/// Kërkesa që pret përgjigje. Numërimi është i dukshëm nga larg, sepse shoferi po ngas:
/// shifra e madhe dhe dy butona, asgjë tjetër për të lexuar (§26).
class OfferCard extends StatefulWidget {
  const OfferCard({super.key, required this.offer});

  final RideOffer offer;

  @override
  State<OfferCard> createState() => _OfferCardState();
}

class _OfferCardState extends State<OfferCard> {
  Timer? _ticker;
  int _secondsLeft = 0;

  @override
  void initState() {
    super.initState();
    _secondsLeft = widget.offer.secondsLeft;
    _ticker = Timer.periodic(const Duration(seconds: 1), (t) {
      if (!mounted) return t.cancel();
      setState(() => _secondsLeft = widget.offer.secondsLeft);
      if (_secondsLeft == 0) t.cancel();
    });
  }

  @override
  void didUpdateWidget(covariant OfferCard old) {
    super.didUpdateWidget(old);
    if (old.offer.id != widget.offer.id) {
      setState(() => _secondsLeft = widget.offer.secondsLeft);
    }
  }

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  Future<void> _accept() async {
    final work = context.read<WorkState>();
    final messenger = ScaffoldMessenger.of(context);
    final gone = context.t('driver.offer.gone');
    final ok = await work.accept(widget.offer);
    if (!mounted) return;
    if (ok) {
      await Navigator.of(context)
          .push(MaterialPageRoute<void>(builder: (_) => const ActiveRideScreen()));
    } else {
      messenger.showSnackBar(SnackBar(content: Text(gone)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final offer = widget.offer;
    final locale = context.watch<AppState>().locale;
    final work = context.watch<WorkState>();
    final expired = _secondsLeft == 0;

    return KCard(
      highlight: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  context.t('driver.offer.new'),
                  style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: K.text),
                ),
              ),
              Text(
                context.t('driver.offer.expires', {'s': '$_secondsLeft'}),
                style: TextStyle(
                  fontSize: 28,
                  fontWeight: FontWeight.w800,
                  color: expired ? K.danger : K.brand400,
                  fontFeatures: const [FontFeature.tabularFigures()],
                ),
              ),
            ],
          ),
          const SizedBox(height: K.s4),
          KMoney(offer.earningsMinor, currency: offer.currency, locale: locale, size: 34),
          Text(
            context.t('driver.offer.earn'),
            style: const TextStyle(fontSize: 12, color: K.muted),
          ),
          const SizedBox(height: K.s4),
          KRow(
            context.t('driver.offer.pickup'),
            '${formatDistance(offer.distanceM, locale: locale)} · ${formatDuration(offer.etaS)}',
          ),
          KRow(
            context.t('driver.offer.trip'),
            '${formatDistance(offer.rideDistanceM, locale: locale)} · '
            '${formatDuration(offer.rideDurationS)}',
          ),
          KRow(context.t('ride.pickup'), offer.pickupAddress ?? '—'),
          KRow(context.t('ride.dropoff'), offer.dropoffAddress ?? '—'),
          KRow(
            context.t('ride.payment'),
            context.t(
              offer.paymentMethod == 'wallet' ? 'ride.payment.wallet' : 'ride.payment.cash',
            ),
          ),
          const SizedBox(height: K.s5),
          KButton(
            label: context.t('driver.offer.accept'),
            icon: Icons.check,
            busy: work.busy,
            onPressed: (expired || work.busy) ? null : _accept,
          ),
          const SizedBox(height: K.s2),
          KOutlineButton(
            label: context.t('driver.offer.decline'),
            onPressed: work.busy ? null : () => context.read<WorkState>().decline(offer),
          ),
        ],
      ),
    );
  }
}
