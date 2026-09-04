import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import '../state/work_state.dart';
import 'active_ride.dart';

/// Kërkesa që pret përgjigje, sipas markës: kufi neon me shkëlqim, çmimi i madh në neon, dy
/// pikat e rrugës (marrja neon, dorëzimi blu), vija e kohës që zbrazet, dhe dy butona
/// krah për krah. Shoferi po ngas: gjithçka lexohet me një shikim (§26).
class OfferCard extends StatefulWidget {
  const OfferCard({super.key, required this.offer});

  final RideOffer offer;

  @override
  State<OfferCard> createState() => _OfferCardState();
}

class _OfferCardState extends State<OfferCard> {
  /// Serveri i jep ofertës 20 s; vija e kohës matet ndaj tyre.
  static const _ttl = 20;

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
    final progress = (_secondsLeft / _ttl).clamp(0.0, 1.0);

    return Container(
      padding: const EdgeInsets.all(K.s4),
      decoration: BoxDecoration(
        color: K.surface,
        borderRadius: BorderRadius.circular(22),
        border: Border.all(color: (expired ? K.danger : K.brand500).withValues(alpha: 0.45)),
        boxShadow: [
          BoxShadow(
            color: (expired ? K.danger : K.brand500).withValues(alpha: 0.10),
            blurRadius: 40,
            offset: const Offset(0, 14),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  context.t('driver.offer.new'),
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: K.text),
                ),
              ),
              Text(
                formatMinor(offer.earningsMinor, locale: locale),
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w800,
                  letterSpacing: -0.4,
                  color: expired ? K.danger : K.brand500,
                  fontFeatures: const [FontFeature.tabularFigures()],
                ),
              ),
              const SizedBox(width: 6),
              KChip(
                context.t(
                  offer.paymentMethod == 'wallet' ? 'ride.payment.wallet' : 'ride.payment.cash',
                ),
              ),
            ],
          ),
          const SizedBox(height: K.s3),
          _RouteRow(
            color: K.brand500,
            glow: true,
            title: offer.pickupAddress ?? context.t('ride.pickup'),
            meta:
                '${context.t('driver.offer.pickup')} · '
                '${formatDistance(offer.distanceM, locale: locale)} · ${formatDuration(offer.etaS)}',
          ),
          const _RouteLine(),
          _RouteRow(
            color: K.info,
            title: offer.dropoffAddress ?? context.t('ride.dropoff'),
            meta:
                '${context.t('driver.offer.trip')} · '
                '${formatDistance(offer.rideDistanceM, locale: locale)} · '
                '${formatDuration(offer.rideDurationS)}',
          ),
          const SizedBox(height: K.s4),
          ClipRRect(
            borderRadius: BorderRadius.circular(4),
            child: TweenAnimationBuilder<double>(
              tween: Tween(end: progress),
              duration: const Duration(milliseconds: 900),
              builder: (_, v, _) => LinearProgressIndicator(
                value: v,
                minHeight: 4,
                color: expired ? K.danger : K.brand500,
                backgroundColor: K.surface3,
              ),
            ),
          ),
          const SizedBox(height: 6),
          Text(
            context.t('driver.offer.expires', {'s': '$_secondsLeft'}),
            textAlign: TextAlign.right,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: expired ? K.danger : K.muted,
              fontFeatures: const [FontFeature.tabularFigures()],
            ),
          ),
          const SizedBox(height: K.s3),
          Row(
            children: [
              Expanded(
                flex: 3,
                child: KButton(
                  label: context.t('driver.offer.accept'),
                  busy: work.busy,
                  onPressed: (expired || work.busy) ? null : _accept,
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                flex: 2,
                child: KOutlineButton(
                  label: context.t('driver.offer.decline'),
                  onPressed: work.busy ? null : () => context.read<WorkState>().decline(offer),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _RouteRow extends StatelessWidget {
  const _RouteRow({
    required this.color,
    required this.title,
    required this.meta,
    this.glow = false,
  });

  final Color color;
  final String title;
  final String meta;
  final bool glow;

  @override
  Widget build(BuildContext context) => Row(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Padding(
        padding: const EdgeInsets.only(top: 5),
        child: Container(
          width: 10,
          height: 10,
          decoration: BoxDecoration(
            color: color,
            shape: BoxShape.circle,
            boxShadow: glow
                ? [BoxShadow(color: color.withValues(alpha: 0.7), blurRadius: 8)]
                : null,
          ),
        ),
      ),
      const SizedBox(width: 12),
      Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: K.text),
            ),
            const SizedBox(height: 2),
            Text(meta, style: const TextStyle(fontSize: 12, color: K.muted)),
          ],
        ),
      ),
    ],
  );
}

class _RouteLine extends StatelessWidget {
  const _RouteLine();

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(left: 4),
    child: Container(width: 2, height: 14, color: K.line2),
  );
}
