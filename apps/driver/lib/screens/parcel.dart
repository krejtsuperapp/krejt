import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import '../state/work_state.dart';

/// Kartela e një pakoje që pret përgjigje: fitimi, dy adresat, numërimi — si te dorëzimet.
class ParcelOfferCard extends StatefulWidget {
  const ParcelOfferCard({super.key, required this.offer});

  final ParcelOffer offer;

  @override
  State<ParcelOfferCard> createState() => _ParcelOfferCardState();
}

class _ParcelOfferCardState extends State<ParcelOfferCard> {
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
  void dispose() {
    _ticker?.cancel();
    super.dispose();
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
                  '${context.t('courier.parcel.offer')} · ${context.t('parcel.size.${offer.size}')}',
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
          KRow(context.t('courier.offer.from'), offer.pickupAddress ?? '—'),
          KRow(context.t('courier.offer.to'), offer.dropoffAddress ?? '—'),
          KRow(
            context.t('driver.offer.pickup'),
            '${formatDistance(offer.distanceM, locale: locale)} · ${formatDuration(offer.etaS)}',
          ),
          KRow(context.t('parcel.route'), formatDistance(offer.routeM, locale: locale)),
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
            onPressed: (expired || work.busy)
                ? null
                : () => context.read<WorkState>().acceptParcel(offer),
          ),
          const SizedBox(height: K.s2),
          KOutlineButton(
            label: context.t('driver.offer.decline'),
            onPressed: work.busy ? null : () => context.read<WorkState>().declineParcel(offer),
          ),
        ],
      ),
    );
  }
}

/// Pakoja në dorëzim: harta me rrugën, marrja me kodin e dërguesit, dorëzimi me kodin e marrësit.
/// Heqja dorë lejohet vetëm para marrjes.
class ActiveParcelScreen extends StatefulWidget {
  const ActiveParcelScreen({super.key});

  @override
  State<ActiveParcelScreen> createState() => _ActiveParcelScreenState();
}

class _ActiveParcelScreenState extends State<ActiveParcelScreen> {
  List<MapPoint>? _path;
  String? _routedFor;

  Future<void> _route(Parcel p) async {
    if (_routedFor == p.id) return;
    _routedFor = p.id;
    try {
      final r = await context.read<AppState>().api.routePath(p.pickup, p.dropoff);
      if (!mounted) return;
      setState(() => _path = [for (final pt in r.points) MapPoint(pt.lat, pt.lng)]);
    } on ApiError {
      _routedFor = null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final work = context.watch<WorkState>();
    final locale = context.watch<AppState>().locale;
    final parcel = work.activeParcel;

    if (parcel == null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (Navigator.of(context).canPop()) Navigator.of(context).pop();
      });
      return Scaffold(
        backgroundColor: K.bg,
        body: Center(child: KLoading(label: context.t('common.loading'))),
      );
    }
    unawaited(_route(parcel));

    final beforePickup = parcel.state != ParcelState.pickedUp;

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('courier.parcel.nav'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            KMap(
              height: 220,
              markers: [
                MapMarker(
                  point: MapPoint(parcel.pickup.lat, parcel.pickup.lng),
                  kind: MapMarkerKind.pickup,
                ),
                MapMarker(
                  point: MapPoint(parcel.dropoff.lat, parcel.dropoff.lng),
                  kind: MapMarkerKind.dropoff,
                ),
              ],
              path: _path,
              schematicCaption: context.t('map.schematic'),
              semanticsLabel: context.t('map.a11y.ride'),
            ),
            const SizedBox(height: K.s4),
            KCard(
              child: Column(
                children: [
                  KRow(context.t('courier.offer.from'), parcel.pickupAddress ?? '—'),
                  if (parcel.pickupContactName != null || parcel.pickupContactPhone != null)
                    KRow(
                      context.t('parcel.sender.name'),
                      [
                        parcel.pickupContactName,
                        parcel.pickupContactPhone,
                      ].whereType<String>().join(' · '),
                    ),
                  KRow(context.t('courier.offer.to'), parcel.dropoffAddress ?? '—'),
                  KRow(
                    context.t('parcel.recipient'),
                    '${parcel.recipientName} · ${parcel.recipientPhone}',
                  ),
                  KRow(context.t('parcel.size'), context.t('parcel.size.${parcel.size}')),
                  if (parcel.note != null) KRow(context.t('parcel.note'), parcel.note!),
                  KRow(
                    context.t('ride.payment'),
                    context.t(
                      parcel.paymentMethod == 'wallet'
                          ? 'ride.payment.wallet'
                          : 'ride.payment.cash',
                    ),
                  ),
                  const KMoneyDivider(),
                  KMoneyRow(
                    context.t('cart.total'),
                    parcel.priceMinor,
                    currency: parcel.currency,
                    locale: locale,
                    total: true,
                  ),
                ],
              ),
            ),
            if (parcel.paymentMethod == 'cash') ...[
              const SizedBox(height: K.s3),
              KCard(
                child: Row(
                  children: [
                    const Icon(Icons.payments_outlined, size: 20, color: K.warn),
                    const SizedBox(width: K.s3),
                    Expanded(
                      child: Text(
                        context.t('driver.ride.cash', {
                          'amount': formatMinor(
                            parcel.priceMinor,
                            currency: parcel.currency,
                            locale: locale,
                          ),
                        }),
                        style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.4),
                      ),
                    ),
                  ],
                ),
              ),
            ],
            if (work.lastError != null) ...[
              const SizedBox(height: K.s3),
              Text(
                context.tError(work.lastError!.messageKey),
                style: const TextStyle(fontSize: 13, color: K.danger),
              ),
            ],
            const SizedBox(height: K.s6),
            if (beforePickup)
              KButton(
                label: context.t('courier.parcel.pickup'),
                icon: Icons.inventory_2_outlined,
                busy: work.busy,
                onPressed: work.busy ? null : () => _withCode(context, work, pickup: true),
              )
            else
              KButton(
                label: context.t('courier.parcel.deliver'),
                icon: Icons.flag_outlined,
                busy: work.busy,
                onPressed: work.busy ? null : () => _withCode(context, work, pickup: false),
              ),
            const SizedBox(height: K.s3),
            if (beforePickup)
              KOutlineButton(
                label: context.t('courier.parcel.release'),
                icon: Icons.undo,
                danger: true,
                onPressed: work.busy
                    ? null
                    : () async {
                        final ok = await confirmKSheet(
                          context: context,
                          title: context.t('courier.release.confirm'),
                          message: context.t('courier.release.body'),
                          confirmLabel: context.t('courier.parcel.release'),
                          cancelLabel: context.t('common.no'),
                          destructive: true,
                        );
                        if (ok) await work.releaseParcel();
                      },
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _withCode(BuildContext context, WorkState work, {required bool pickup}) async {
    final controller = TextEditingController();
    final code = await showKSheet<String>(
      context: context,
      title: context.t(pickup ? 'courier.parcel.pickup' : 'courier.parcel.deliver'),
      subtitle: context.t(pickup ? 'courier.parcel.pickup.hint' : 'courier.parcel.deliver.hint'),
      child: Column(
        children: [
          KField(
            label: context.t('courier.parcel.code'),
            controller: controller,
            hint: '1234',
            maxLength: 4,
            autofocus: true,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            textInputAction: TextInputAction.done,
            onSubmitted: (v) => Navigator.of(context).pop(v.trim()),
          ),
          const SizedBox(height: K.s4),
          KButton(
            label: context.t(pickup ? 'courier.parcel.pickup' : 'courier.parcel.deliver'),
            onPressed: () => Navigator.of(context).pop(controller.text.trim()),
          ),
        ],
      ),
    );
    controller.dispose();
    if (code == null || code.length != 4) return;
    if (pickup) {
      await work.pickupParcel(code: code);
    } else {
      await work.deliverParcel(code: code);
    }
  }
}
