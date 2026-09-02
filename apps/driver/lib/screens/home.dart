import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Paneli i shoferit. Butoni i madh është i vetmi vendim i ekranit: në punë ose jashtë pune.
/// Kur llogaria nuk është aprovuar, arsyeja shfaqet këtu në vend të butonit (§27).
class DriverHomeScreen extends StatefulWidget {
  const DriverHomeScreen({super.key});

  @override
  State<DriverHomeScreen> createState() => _DriverHomeScreenState();
}

class _DriverHomeScreenState extends State<DriverHomeScreen> {
  bool _busy = false;
  String? _error;

  Future<void> _toggle() async {
    final state = context.read<AppState>();
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await state.setOnline(!state.online);
    } on ApiError catch (e) {
      if (mounted) setState(() => _error = context.tError(e.messageKey));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final driver = state.driver;
    final approved = state.canGoOnline;

    return Scaffold(
      backgroundColor: K.bg,
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: state.refreshDriver,
          color: K.brand400,
          backgroundColor: K.surface2,
          child: ListView(
            padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      state.me?.displayName ?? '—',
                      style: const TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                        color: K.text,
                      ),
                    ),
                  ),
                  KBadge(
                    context.t(state.online ? 'driver.online' : 'driver.offline'),
                    tone: state.online ? KTone.ok : KTone.neutral,
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
              if (!approved)
                KCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        context.t('driver.pending'),
                        style: const TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                          color: K.text,
                        ),
                      ),
                      const SizedBox(height: K.s2),
                      Text(
                        driver?.suspendedReason ?? context.t('driver.pending.hint'),
                        style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.45),
                      ),
                    ],
                  ),
                )
              else
                KButton(
                  label: context.t(state.online ? 'driver.go_offline' : 'driver.go_online'),
                  icon: state.online ? Icons.pause_circle_outline : Icons.play_circle_outline,
                  busy: _busy,
                  danger: state.online,
                  onPressed: _busy ? null : _toggle,
                ),
              if (_error != null) ...[
                const SizedBox(height: K.s3),
                Text(_error!, style: const TextStyle(fontSize: 13, color: K.danger)),
              ],
              const SizedBox(height: K.s6),
              KSectionHeader(context.t('driver.earnings')),
              const SizedBox(height: K.s3),
              // Fitimet kanë kuptim vetëm pasi ekziston profili i shoferit; pa të nuk e pyesim serverin.
              if (driver == null)
                KEmpty(
                  title: context.t('state.empty'),
                  message: context.t('driver.pending.hint'),
                  icon: Icons.payments_outlined,
                )
              else
                const _EarningsCard(),
              const SizedBox(height: K.s5),
              KOutlineButton(
                label: context.t('auth.logout'),
                icon: Icons.logout,
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
        ),
      ),
    );
  }
}

/// Fitimet lexohen nga serveri sa herë hapet ekrani; asnjë shifër nuk llogaritet në pajisje.
class _EarningsCard extends StatefulWidget {
  const _EarningsCard();

  @override
  State<_EarningsCard> createState() => _EarningsCardState();
}

class _EarningsCardState extends State<_EarningsCard> {
  Future<Earnings>? _future;

  @override
  void initState() {
    super.initState();
    _future = context.read<AppState>().api.earnings();
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    return FutureBuilder<Earnings>(
      future: _future,
      builder: (context, snap) {
        if (snap.connectionState == ConnectionState.waiting) {
          return const KSkeleton(height: 96, count: 1);
        }
        if (snap.hasError || !snap.hasData) {
          final e = snap.error;
          return KError(
            message: e is ApiError ? context.tError(e.messageKey) : context.t('errors.internal'),
            retryLabel: context.t('common.retry'),
            onRetry: () => setState(() => _future = context.read<AppState>().api.earnings()),
          );
        }
        final d = snap.data!;
        return KCard(
          child: Column(
            children: [
              KMoneyRow(context.t('driver.earnings.today'), d.todayMinor, locale: locale),
              KMoneyRow(context.t('driver.earnings.week'), d.weekMinor, locale: locale),
              const KMoneyDivider(),
              KMoneyRow(context.t('driver.payout'), d.balanceMinor, locale: locale, total: true),
            ],
          ),
        );
      },
    );
  }
}
