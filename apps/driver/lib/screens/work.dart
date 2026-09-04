import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../services/location.dart';
import '../state/app_state.dart';
import '../state/work_state.dart';
import 'active_ride.dart';
import 'apply.dart';
import 'courier.dart';
import 'documents.dart';
import 'offer_card.dart';
import 'parcel.dart';

/// Ekrani i punës sipas markës: "Sot" me ndërruesin neon të turnit në krye, tri shifrat e
/// ditës (nga serveri), pastaj ose kërkesa që pret përgjigje, ose udhëtimi në rrjedhë, ose
/// arsyeja pse asnjëra nuk ndodh (§27).
class WorkScreen extends StatelessWidget {
  const WorkScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final work = context.watch<WorkState>();
    final driver = state.driver;
    final earnings = state.earnings;
    final locale = state.locale;
    final ride = work.activeRide;
    final order = work.activeOrder;
    final parcel = work.activeParcel;
    final offer = work.topOffer;
    final delivery = work.topDeliveryOffer;
    final parcelOffer = work.topParcelOffer;

    return SafeArea(
      child: ListView(
        padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, K.s8),
        children: [
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      driver == null
                          ? (state.me?.displayName ?? '—')
                          : '${driver.vehicle} · ${driver.vehiclePlate}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 13, color: K.muted),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      context.t('driver.today'),
                      style: const TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.w800,
                        letterSpacing: -0.4,
                        color: K.text,
                      ),
                    ),
                  ],
                ),
              ),
              if (state.canGoOnline)
                _OnlinePill(work: work, categories: driver!.categories)
              else
                KPill(context.t('driver.offline'), icon: Icons.pause_circle_outline),
            ],
          ),
          if (earnings != null) ...[
            const SizedBox(height: K.s4),
            Row(
              children: [
                Expanded(
                  child: KKpi(
                    label: context.t('driver.kpi.earnings'),
                    value: formatMinor(earnings.todayMinor, locale: locale),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: KKpi(
                    label: context.t('driver.kpi.rides'),
                    value: '${earnings.ridesToday}',
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: KKpi(
                    label: context.t('driver.kpi.rating'),
                    value: driver?.rating == null ? '—' : driver!.rating!.toStringAsFixed(1),
                    accent: driver?.rating == null ? null : '★',
                  ),
                ),
              ],
            ),
          ],
          const SizedBox(height: K.s4),
          if (driver == null)
            const _ApplyCard()
          else if (!state.canGoOnline)
            _NotApprovedCard(reason: driver.suspendedReason),
          if (work.locationProblem != null) ...[
            KError(
              message: context.t(locationProblemKey(work.locationProblem!)),
              icon: Icons.location_off_outlined,
            ),
            const SizedBox(height: K.s4),
          ],
          if (ride != null)
            KNeonBanner(
              icon: Icons.directions_car_outlined,
              title: context.t('driver.status.busy'),
              subtitle: context.t(driverRideStateKey(ride.state)),
              onTap: () =>
                  Navigator.of(context)
                      .push(MaterialPageRoute<void>(builder: (_) => const ActiveRideScreen())),
            )
          else if (order != null)
            KNeonBanner(
              icon: Icons.delivery_dining_outlined,
              title: context.t('courier.nav'),
              subtitle: context.t(courierOrderStateKey(order.state)),
              onTap: () =>
                  Navigator.of(context)
                      .push(MaterialPageRoute<void>(builder: (_) => const ActiveDeliveryScreen())),
            )
          else if (parcel != null)
            KNeonBanner(
              icon: Icons.inventory_2_outlined,
              title: context.t('courier.parcel.nav'),
              subtitle: context.t(
                parcel.state == ParcelState.pickedUp
                    ? 'parcel.state.picked_up'
                    : 'parcel.state.courier_assigned',
              ),
              onTap: () =>
                  Navigator.of(context)
                      .push(MaterialPageRoute<void>(builder: (_) => const ActiveParcelScreen())),
            )
          else if (offer != null)
            OfferCard(offer: offer)
          else if (delivery != null)
            CourierOfferCard(offer: delivery)
          else if (parcelOffer != null)
            ParcelOfferCard(offer: parcelOffer)
          else if (work.online)
            KEmpty(
              title: context.t('driver.status.waiting'),
              message: context.t('driver.status.waiting.hint'),
              icon: Icons.notifications_active_outlined,
            )
          else if (state.canGoOnline)
            // Kur llogaria nuk është aprovuar, karta lart e ka shpjeguar tashmë arsyen;
            // përsëritja e saj këtu do të ishte zhurmë.
            KEmpty(
              title: context.t('driver.status.offline.hint'),
              icon: Icons.pause_circle_outline,
            ),
          const SizedBox(height: K.s5),
          KOutlineButton(
            label: context.t('driver.docs.title'),
            icon: Icons.description_outlined,
            onPressed: () =>
                Navigator.of(context)
                    .push(MaterialPageRoute<void>(builder: (_) => const DocumentsScreen())),
          ),
        ],
      ),
    );
  }
}

/// Gjendja e një dorëzimi, e parë nga korrieri: para marrjes dhe pas saj.
String courierOrderStateKey(OrderState s) =>
    s == OrderState.pickedUp ? 'order.state.picked_up' : 'order.state.ready';

String driverRideStateKey(RideState s) {
  switch (s) {
    case RideState.assigned:
      return 'driver.ride.to_pickup';
    case RideState.arrived:
      return 'driver.ride.waiting';
    case RideState.inProgress:
      return 'driver.ride.driving';
    default:
      return 'ride.state.matching';
  }
}

/// Ndërruesi i turnit si pill: neon kur je në punë (prekja të nxjerr), kontur kur je jashtë.
class _OnlinePill extends StatelessWidget {
  const _OnlinePill({required this.work, required this.categories});

  final WorkState work;
  final List<RideCategory> categories;

  @override
  Widget build(BuildContext context) {
    final on = work.online;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: work.busy
            ? null
            : () async {
                if (on) {
                  await work.goOffline();
                } else {
                  await work.goOnline(categories.map((c) => c.name).toList());
                }
              },
        borderRadius: BorderRadius.circular(K.rFull),
        child: Ink(
          height: 40,
          padding: const EdgeInsets.symmetric(horizontal: 14),
          decoration: BoxDecoration(
            color: on ? K.brand500 : K.surface,
            borderRadius: BorderRadius.circular(K.rFull),
            border: Border.all(color: on ? K.brand500 : K.line2),
            boxShadow: on
                ? [BoxShadow(color: K.brand500.withValues(alpha: 0.35), blurRadius: 20)]
                : null,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (work.busy)
                SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: on ? K.onBrand : K.brand500,
                  ),
                )
              else
                Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(
                    color: on ? K.onBrand : K.muted,
                    shape: BoxShape.circle,
                  ),
                ),
              const SizedBox(width: 8),
              Text(
                context.t(on ? 'driver.online' : 'driver.go_online'),
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w700,
                  color: on ? K.onBrand : K.text,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Përdoruesi ende s'ka aplikuar: një kartë e vetme me hapin e parë, pa "në pritje" të rremë.
class _ApplyCard extends StatelessWidget {
  const _ApplyCard();

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: K.s4),
    child: KCard(
      highlight: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            context.t('driver.apply.card.title'),
            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: K.text),
          ),
          const SizedBox(height: K.s2),
          Text(
            context.t('driver.apply.card.hint'),
            style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.45),
          ),
          const SizedBox(height: K.s4),
          KButton(
            label: context.t('driver.apply.card.action'),
            icon: Icons.arrow_forward,
            onPressed: () =>
                Navigator.of(context)
                    .push(MaterialPageRoute<void>(builder: (_) => const ApplyScreen())),
          ),
        ],
      ),
    ),
  );
}

class _NotApprovedCard extends StatelessWidget {
  const _NotApprovedCard({this.reason});

  final String? reason;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: K.s4),
    child: KCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            context.t('driver.pending'),
            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: K.text),
          ),
          const SizedBox(height: K.s2),
          Text(
            reason ?? context.t('driver.pending.hint'),
            style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.45),
          ),
        ],
      ),
    ),
  );
}
