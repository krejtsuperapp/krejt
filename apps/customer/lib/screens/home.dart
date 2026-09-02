import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Çelësi i përkthimit për një gjendje udhëtimi, i mbajtur në një vend të vetëm
/// që asnjë ekran të mos shpikë etiketat e veta.
String rideStateKey(RideState s) {
  switch (s) {
    case RideState.matching:
      return 'ride.state.matching';
    case RideState.assigned:
      return 'ride.state.assigned';
    case RideState.arrived:
      return 'ride.state.arrived';
    case RideState.inProgress:
      return 'ride.state.in_progress';
    case RideState.completed:
      return 'ride.state.completed';
    case RideState.cancelled:
      return 'ride.state.cancelled';
    case RideState.noDriver:
      return 'ride.state.no_driver';
  }
}

/// Ballina: udhëtimi aktiv në krye nëse ka, pastaj bilanci, shërbimet dhe historiku i shkurtër.
/// Shërbimet që nuk janë ndezur në konfigurim shfaqen si të ardhshme, jo të fshehura (§55, §64).
class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final me = state.me;
    final locale = state.locale;
    final active = state.activeRide;
    final past = state.recentRides.where((r) => r.isFinished).take(4).toList();

    return SafeArea(
      child: RefreshIndicator(
        onRefresh: state.refreshHome,
        color: K.brand400,
        backgroundColor: K.surface2,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            Text(
              context.t('home.greeting', {'name': me?.displayName ?? '—'}),
              style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: K.text),
            ),
            const SizedBox(height: K.s4),
            if (active != null) ...[
              KActiveBanner(
                icon: Icons.local_taxi,
                title: context.t('home.active.ride'),
                subtitle: context.t(rideStateKey(active.state)),
              ),
              const SizedBox(height: K.s4),
            ],
            _WalletCard(balanceMinor: me?.wallet.balanceMinor ?? 0, locale: locale),
            const SizedBox(height: K.s5),
            KSectionHeader(context.t('home.where')),
            const SizedBox(height: K.s3),
            Row(
              children: [
                Expanded(
                  child: _ServiceTile(
                    icon: Icons.local_taxi_outlined,
                    label: context.t('home.services.ride'),
                    ready: state.config.flag('rides', fallback: true),
                  ),
                ),
                const SizedBox(width: K.s3),
                Expanded(
                  child: _ServiceTile(
                    icon: Icons.restaurant_outlined,
                    label: context.t('home.services.food'),
                    ready: state.config.flag('food'),
                  ),
                ),
              ],
            ),
            const SizedBox(height: K.s3),
            Row(
              children: [
                Expanded(
                  child: _ServiceTile(
                    icon: Icons.storefront_outlined,
                    label: context.t('home.services.market'),
                    ready: state.config.flag('market'),
                  ),
                ),
                const SizedBox(width: K.s3),
                Expanded(
                  child: _ServiceTile(
                    icon: Icons.account_balance_wallet_outlined,
                    label: context.t('home.services.wallet'),
                    ready: true,
                  ),
                ),
              ],
            ),
            const SizedBox(height: K.s6),
            KSectionHeader(context.t('home.recent')),
            const SizedBox(height: K.s3),
            if (past.isEmpty)
              KEmpty(
                title: context.t('home.rides.empty'),
                message: context.t('home.rides.empty.hint'),
                icon: Icons.route_outlined,
              )
            else
              for (final r in past)
                Padding(
                  padding: const EdgeInsets.only(bottom: K.s2),
                  child: _RideRow(ride: r, locale: locale),
                ),
          ],
        ),
      ),
    );
  }
}

class _WalletCard extends StatelessWidget {
  const _WalletCard({required this.balanceMinor, required this.locale});

  final int balanceMinor;
  final String locale;

  @override
  Widget build(BuildContext context) => KCard(
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          context.t('wallet.balance'),
          style: const TextStyle(fontSize: 13, color: K.muted, fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: K.s2),
        KMoney(balanceMinor, locale: locale, size: 32),
        const SizedBox(height: K.s3),
        Text(
          context.t('wallet.closed_loop'),
          style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
        ),
      ],
    ),
  );
}

class _ServiceTile extends StatelessWidget {
  const _ServiceTile({required this.icon, required this.label, required this.ready});

  final IconData icon;
  final String label;
  final bool ready;

  @override
  Widget build(BuildContext context) => KCard(
    padding: const EdgeInsets.all(K.s4),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 26, color: ready ? K.brand400 : K.muted),
        const SizedBox(height: K.s3),
        Text(
          label,
          style: TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.w600,
            color: ready ? K.text : K.muted,
          ),
        ),
        if (!ready) ...[
          const SizedBox(height: K.s2),
          KBadge(context.t('common.soon'), tone: KTone.neutral),
        ],
      ],
    ),
  );
}

class _RideRow extends StatelessWidget {
  const _RideRow({required this.ride, required this.locale});

  final Ride ride;
  final String locale;

  @override
  Widget build(BuildContext context) {
    final where = ride.dropoffAddress ?? formatDistance(ride.distanceM, locale: locale);
    return KCard(
      padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  where,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: K.text),
                ),
                const SizedBox(height: 2),
                Text(
                  context.t(rideStateKey(ride.state)),
                  style: const TextStyle(fontSize: 12, color: K.muted),
                ),
              ],
            ),
          ),
          const SizedBox(width: K.s3),
          KMoney(
            ride.priceMinor,
            locale: locale,
            size: 16,
            strikethrough: ride.state == RideState.cancelled,
          ),
        ],
      ),
    );
  }
}
