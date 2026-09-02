import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../services/location.dart';
import '../state/app_state.dart';
import '../state/work_state.dart';
import 'active_ride.dart';
import 'courier.dart';
import 'documents.dart';
import 'offer_card.dart';

/// Ekrani i punës. Një vendim i madh në krye: në punë ose jashtë pune. Nën të, ose kërkesa
/// që pret përgjigje, ose udhëtimi në rrjedhë, ose arsyeja pse asnjëra nuk ndodh (§27).
class WorkScreen extends StatelessWidget {
  const WorkScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final work = context.watch<WorkState>();
    final driver = state.driver;
    final ride = work.activeRide;
    final order = work.activeOrder;
    final offer = work.topOffer;
    final delivery = work.topDeliveryOffer;

    return SafeArea(
      child: ListView(
        padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  state.me?.displayName ?? '—',
                  style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: K.text),
                ),
              ),
              KBadge(
                context.t(work.online ? 'driver.online' : 'driver.offline'),
                tone: work.online ? KTone.ok : KTone.neutral,
              ),
            ],
          ),
          if (driver != null) ...[
            const SizedBox(height: K.s1),
            Text(
              '${driver.vehicle} · ${driver.vehiclePlate}',
              style: const TextStyle(fontSize: 13, color: K.muted),
            ),
          ],
          const SizedBox(height: K.s5),
          if (!state.canGoOnline)
            _NotApprovedCard(reason: driver?.suspendedReason)
          else
            _OnlineToggle(work: work, categories: driver!.categories),
          if (work.locationProblem != null) ...[
            const SizedBox(height: K.s3),
            KError(
              message: context.t(locationProblemKey(work.locationProblem!)),
              icon: Icons.location_off_outlined,
            ),
          ],
          const SizedBox(height: K.s5),
          if (ride != null)
            KActiveBanner(
              icon: Icons.local_taxi,
              title: context.t('driver.status.busy'),
              subtitle: context.t(driverRideStateKey(ride.state)),
              onTap: () =>
                  Navigator.of(context)
                      .push(MaterialPageRoute<void>(builder: (_) => const ActiveRideScreen())),
            )
          else if (order != null)
            KActiveBanner(
              icon: Icons.delivery_dining,
              title: context.t('courier.nav'),
              subtitle: context.t(courierOrderStateKey(order.state)),
              onTap: () =>
                  Navigator.of(context)
                      .push(MaterialPageRoute<void>(builder: (_) => const ActiveDeliveryScreen())),
            )
          else if (offer != null)
            OfferCard(offer: offer)
          else if (delivery != null)
            CourierOfferCard(offer: delivery)
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

class _OnlineToggle extends StatelessWidget {
  const _OnlineToggle({required this.work, required this.categories});

  final WorkState work;
  final List<RideCategory> categories;

  @override
  Widget build(BuildContext context) => KButton(
    label: context.t(work.online ? 'driver.go_offline' : 'driver.go_online'),
    icon: work.online ? Icons.pause_circle_outline : Icons.play_circle_outline,
    busy: work.busy,
    danger: work.online,
    onPressed: work.busy
        ? null
        : () async {
            if (work.online) {
              await work.goOffline();
            } else {
              await work.goOnline(categories.map((c) => c.name).toList());
            }
          },
  );
}

class _NotApprovedCard extends StatelessWidget {
  const _NotApprovedCard({this.reason});

  final String? reason;

  @override
  Widget build(BuildContext context) => KCard(
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
  );
}
