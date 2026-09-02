import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import '../state/work_state.dart';

/// Kartela e një dorëzimi që pret përgjigje. Njësoj si te udhëtimet: numërim i madh,
/// fitimi, dhe dy përgjigje. Ndryshimi i vetëm është se ka dy adresa, jo një (§26).
class CourierOfferCard extends StatefulWidget {
  const CourierOfferCard({super.key, required this.offer});

  final CourierOffer offer;

  @override
  State<CourierOfferCard> createState() => _CourierOfferCardState();
}

class _CourierOfferCardState extends State<CourierOfferCard> {
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
                  context.t('courier.offer.new'),
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
          KRow(context.t('courier.offer.from'), '${offer.merchantName} · ${offer.merchantAddress}'),
          KRow(context.t('courier.offer.to'), offer.dropoffAddress ?? '—'),
          KRow(
            context.t('driver.offer.pickup'),
            '${formatDistance(offer.distanceM, locale: locale)} · ${formatDuration(offer.etaS)}',
          ),
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
                : () => context.read<WorkState>().acceptDelivery(offer),
          ),
          const SizedBox(height: K.s2),
          KOutlineButton(
            label: context.t('driver.offer.decline'),
            onPressed: work.busy ? null : () => context.read<WorkState>().declineDelivery(offer),
          ),
        ],
      ),
    );
  }
}

/// Dorëzimi në rrjedhë: merr te vendi me kodin gjashtështkronjor, pastaj dorëzo.
/// Heqja dorë lejohet vetëm para marrjes; pas saj porosia është në duart e korrierit (§26).
class ActiveDeliveryScreen extends StatelessWidget {
  const ActiveDeliveryScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final work = context.watch<WorkState>();
    final locale = context.watch<AppState>().locale;
    final order = work.activeOrder;

    if (order == null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (Navigator.of(context).canPop()) Navigator.of(context).pop();
      });
      return Scaffold(
        backgroundColor: K.bg,
        body: Center(child: KLoading(label: context.t('common.loading'))),
      );
    }

    final beforePickup = order.state != OrderState.pickedUp;

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(order.merchantName)),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            KCard(
              child: Column(
                children: [
                  KRow(context.t('courier.offer.from'), order.merchantName),
                  KRow(context.t('courier.offer.to'), order.addressText ?? '—'),
                  KRow(
                    context.t('ride.payment'),
                    context.t(
                      order.paymentMethod == 'wallet' ? 'ride.payment.wallet' : 'ride.payment.cash',
                    ),
                  ),
                  const KMoneyDivider(),
                  KMoneyRow(
                    context.t('cart.total'),
                    order.totalMinor,
                    currency: order.currency,
                    locale: locale,
                    total: true,
                  ),
                ],
              ),
            ),
            if (order.paymentMethod == 'cash') ...[
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
                            order.totalMinor,
                            currency: order.currency,
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
                label: context.t('courier.pickup'),
                icon: Icons.inventory_2_outlined,
                busy: work.busy,
                onPressed: work.busy ? null : () => _pickup(context, work),
              )
            else
              KButton(
                label: context.t('courier.deliver'),
                icon: Icons.flag_outlined,
                busy: work.busy,
                onPressed: work.busy ? null : work.deliver,
              ),
            const SizedBox(height: K.s3),
            if (beforePickup)
              KOutlineButton(
                label: context.t('courier.release'),
                icon: Icons.undo,
                danger: true,
                onPressed: work.busy
                    ? null
                    : () async {
                        final ok = await confirmKSheet(
                          context: context,
                          title: context.t('courier.release.confirm'),
                          message: context.t('courier.release.body'),
                          confirmLabel: context.t('courier.release'),
                          cancelLabel: context.t('common.no'),
                          destructive: true,
                        );
                        if (ok) await work.release();
                      },
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _pickup(BuildContext context, WorkState work) async {
    final controller = TextEditingController();
    final code = await showKSheet<String>(
      context: context,
      title: context.t('courier.pickup.code'),
      subtitle: context.t('courier.pickup.hint'),
      child: Column(
        children: [
          KField(
            label: context.t('courier.pickup.code'),
            controller: controller,
            hint: 'K7F3QA',
            maxLength: 6,
            autofocus: true,
            textInputAction: TextInputAction.done,
            onSubmitted: (v) => Navigator.of(context).pop(v.trim().toUpperCase()),
          ),
          const SizedBox(height: K.s4),
          KButton(
            label: context.t('courier.pickup'),
            onPressed: () => Navigator.of(context).pop(controller.text.trim().toUpperCase()),
          ),
        ],
      ),
    );
    controller.dispose();
    if (code == null || code.length != 6) return;
    await work.pickup(code: code);
  }
}
