import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

import 'package:krejt_screens/krejt_screens.dart';

import '../services/location.dart';
import '../state/app_state.dart';
import '../state/work_state.dart';
import 'work.dart';

/// Udhëtimi në rrjedhë, hap pas hapi. Në çdo çast ka vetëm një veprim kryesor,
/// sepse shoferi po ngas: mbërrita, nis, përfundo (§18, §25).
class ActiveRideScreen extends StatelessWidget {
  const ActiveRideScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final work = context.watch<WorkState>();
    final locale = context.watch<AppState>().locale;
    final ride = work.activeRide;

    if (ride == null) {
      // Udhëtimi mbaroi ndërkohë; ekrani mbyllet vetë në kuadrin e radhës.
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (Navigator.of(context).canPop()) Navigator.of(context).pop();
      });
      return Scaffold(
        backgroundColor: K.bg,
        body: Center(child: KLoading(label: context.t('common.loading'))),
      );
    }

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(
        title: Text(context.t(driverRideStateKey(ride.state))),
        // Poshtë rri veprimi kryesor, që ndryshon me gjendjen; një buton sigurie pranë tij do
        // të lëvizte bashkë me të dhe do të ftonte prekje të gabuar — dhe shoferi po ngas.
        actions: [
          IconButton(
            icon: const Icon(Icons.shield_outlined),
            tooltip: context.t('safety.title'),
            onPressed: () => _safety(context, ride.id),
          ),
        ],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            KMap(
              height: 170,
              markers: [
                MapMarker(
                  point: MapPoint(ride.pickup.lat, ride.pickup.lng),
                  kind: MapMarkerKind.pickup,
                ),
                MapMarker(
                  point: MapPoint(ride.dropoff.lat, ride.dropoff.lng),
                  kind: MapMarkerKind.dropoff,
                ),
              ],
              schematicCaption: context.t('map.schematic'),
              semanticsLabel: context.t('map.a11y.ride'),
            ),
            const SizedBox(height: K.s4),
            KCard(
              child: Column(
                children: [
                  KRow(context.t('ride.pickup'), ride.pickupAddress ?? '—'),
                  KRow(context.t('ride.dropoff'), ride.dropoffAddress ?? '—'),
                  KRow(
                    context.t('ride.summary'),
                    '${formatDistance(ride.distanceM, locale: locale)} · '
                    '${formatDuration(ride.durationS)}',
                  ),
                  const KMoneyDivider(),
                  KMoneyRow(
                    context.t('driver.offer.earn'),
                    ride.priceMinor,
                    currency: ride.currency,
                    locale: locale,
                    total: true,
                  ),
                ],
              ),
            ),
            const SizedBox(height: K.s4),
            KCard(
              child: Row(
                children: [
                  Icon(
                    ride.paymentMethod == 'cash'
                        ? Icons.payments_outlined
                        : Icons.account_balance_wallet_outlined,
                    size: 20,
                    color: ride.paymentMethod == 'cash' ? K.warn : K.ok,
                  ),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Text(
                      ride.paymentMethod == 'cash'
                          ? context.t('driver.ride.cash', {
                              'amount': formatMinor(
                                ride.priceMinor,
                                currency: ride.currency,
                                locale: locale,
                              ),
                            })
                          : context.t('driver.ride.paid'),
                      style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.4),
                    ),
                  ),
                ],
              ),
            ),
            if (work.lastError != null) ...[
              const SizedBox(height: K.s3),
              Text(
                context.tError(work.lastError!.messageKey),
                style: const TextStyle(fontSize: 13, color: K.danger),
              ),
            ],
            const SizedBox(height: K.s6),
            _PrimaryAction(ride: ride, work: work),
            const SizedBox(height: K.s3),
            if (ride.state != RideState.inProgress)
              KOutlineButton(
                label: context.t('driver.ride.cancel'),
                icon: Icons.close,
                danger: true,
                onPressed: work.busy
                    ? null
                    : () async {
                        final ok = await confirmKSheet(
                          context: context,
                          title: context.t('driver.ride.cancel.confirm'),
                          message: context.t('driver.ride.cancel.body'),
                          confirmLabel: context.t('driver.ride.cancel'),
                          cancelLabel: context.t('common.no'),
                          destructive: true,
                        );
                        if (ok) await work.cancelRide();
                      },
              ),
          ],
        ),
      ),
    );
  }
}

/// Vendndodhja merret këtu: paketa e përbashkët nuk i njeh shërbimet e secilit aplikacion, dhe
/// një leje e mohuar nuk duhet ta ndalë raportin.
Future<void> _safety(BuildContext context, String rideId) async {
  final api = context.read<AppState>().api;
  final position = await const LocationService().current();
  if (!context.mounted) return;
  await showSafetySheet(context, api: api, rideId: rideId, at: position.point);
}

class _PrimaryAction extends StatelessWidget {
  const _PrimaryAction({required this.ride, required this.work});

  final Ride ride;
  final WorkState work;

  Future<void> _start(BuildContext context) async {
    final code = await showKSheet<String>(
      context: context,
      title: context.t('driver.ride.code.title'),
      subtitle: context.t('driver.ride.code.hint'),
      child: const _CodeEntry(),
    );
    if (code == null || !context.mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    final wrong = context.t('driver.ride.code.wrong');
    final ok = await work.start(pickupCode: code);
    if (!ok) messenger.showSnackBar(SnackBar(content: Text(wrong)));
  }

  @override
  Widget build(BuildContext context) {
    switch (ride.state) {
      case RideState.assigned:
        return KButton(
          label: context.t('driver.arrived'),
          icon: Icons.location_on_outlined,
          busy: work.busy,
          onPressed: work.busy ? null : work.arrived,
        );
      case RideState.arrived:
        return KButton(
          label: context.t('driver.start'),
          icon: Icons.play_arrow,
          busy: work.busy,
          onPressed: work.busy ? null : () => _start(context),
        );
      case RideState.inProgress:
        return KButton(
          label: context.t('driver.complete'),
          icon: Icons.flag_outlined,
          busy: work.busy,
          onPressed: work.busy ? null : work.complete,
        );
      default:
        return KLoading(label: context.t('common.loading'));
    }
  }
}

/// Katër shifrat që i thotë klienti. Pa to udhëtimi nuk nis, që askush të mos hipë në makinën e gabuar.
class _CodeEntry extends StatefulWidget {
  const _CodeEntry();

  @override
  State<_CodeEntry> createState() => _CodeEntryState();
}

class _CodeEntryState extends State<_CodeEntry> {
  @override
  Widget build(BuildContext context) =>
      KOtpField(length: 4, onCompleted: (code) => Navigator.of(context).pop(code));
}
