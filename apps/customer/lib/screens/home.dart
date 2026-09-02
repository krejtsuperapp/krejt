import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Shtëpia e klientit: kush je, sa ke, ku po shkon. Shërbimet që s'janë gati ende
/// shfaqen si të ardhshme në vend që të fshihen ose të gënjejnë (§55).
class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final me = state.me;
    final locale = state.locale;

    return Scaffold(
      backgroundColor: K.bg,
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: state.refreshMe,
          color: K.brand400,
          backgroundColor: K.surface2,
          child: ListView(
            padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      context.t('home.greeting', {'name': me?.displayName ?? '—'}),
                      style: const TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.w700,
                        color: K.text,
                      ),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.logout, color: K.muted),
                    tooltip: context.t('auth.logout'),
                    onPressed: () async {
                      final ok = await confirmKSheet(
                        context: context,
                        title: context.t('auth.logout.confirm'),
                        message: context.t('auth.logout'),
                        confirmLabel: context.t('auth.logout'),
                        cancelLabel: context.t('common.no'),
                        destructive: true,
                      );
                      if (ok) await state.signOut();
                    },
                  ),
                ],
              ),
              const SizedBox(height: K.s4),
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
            ],
          ),
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
