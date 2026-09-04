import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import 'food/order_tracking.dart';
import 'home.dart' show rideStateKey;
import 'parcels/parcel_tracking.dart';
import 'ride/tracking.dart';
import 'services/request_tracking.dart';

/// «Ke diçka në rrjedhë këtu» brenda vetë degës.
///
/// Ballina i tregon të katërta, por aty arrin vetëm kur nis prej saj. Kush hyn te Korrieri nga
/// Aktiviteti, nga një njoftim ose nga një deep link, e gjente formularin bosh sikur pakoja e tij
/// të mos ekzistonte — dhe rruga e vetme mbrapa ishte dalja te ballina.
///
/// Banderola nuk e rrëmben lundrimin: forma mbetet poshtë saj. Një pako në rrugë nuk do të thotë
/// se përdoruesi nuk mund të dërgojë një tjetër; do të thotë vetëm se e para nuk duhet t'i humbë.
enum ActiveKind { ride, order, parcel, service }

class ActiveBanner extends StatelessWidget {
  const ActiveBanner({super.key, required this.kind});

  final ActiveKind kind;

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final (
      IconData icon,
      String titleKey,
      String? subtitle,
      Widget Function() open,
    ) = switch (kind) {
      ActiveKind.ride => (
        Icons.directions_car_outlined,
        'home.active.ride',
        state.activeRide == null ? null : context.t(rideStateKey(state.activeRide!.state)),
        () => TrackingScreen(rideId: state.activeRide!.id),
      ),
      ActiveKind.order => (
        Icons.lunch_dining_outlined,
        'home.active.order',
        state.activeOrder == null ? null : context.t(orderStateKey(state.activeOrder!.state)),
        () => OrderTrackingScreen(orderId: state.activeOrder!.id),
      ),
      ActiveKind.parcel => (
        Icons.inventory_2_outlined,
        'parcel.active',
        state.activeParcel == null ? null : context.t(parcelStateKey(state.activeParcel!.state)),
        () => ParcelTrackingScreen(parcelId: state.activeParcel!.id),
      ),
      ActiveKind.service => (
        Icons.handyman_outlined,
        'service.active',
        state.activeService == null ? null : context.t(serviceStateKey(state.activeService!.state)),
        () => ServiceTrackingScreen(requestId: state.activeService!.id),
      ),
    };
    if (subtitle == null) return const SizedBox.shrink();

    return Padding(
      padding: const EdgeInsets.only(bottom: K.s4),
      child: KNeonBanner(
        icon: icon,
        title: context.t(titleKey),
        subtitle: subtitle,
        onTap: () => Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => open())),
      ),
    );
  }
}
