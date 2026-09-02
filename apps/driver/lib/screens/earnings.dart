import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import 'bank.dart';

/// Fitimet vijnë të llogaritura nga serveri. Aplikacioni nuk mbledh dhe nuk zbret asgjë,
/// që shifra në ekran të jetë e njëjta me atë të pagesës javore (§23).
class EarningsScreen extends StatefulWidget {
  const EarningsScreen({super.key});

  @override
  State<EarningsScreen> createState() => _EarningsScreenState();
}

class _EarningsScreenState extends State<EarningsScreen> {
  Earnings? _earnings;
  BankAccount? _bank;
  bool _loading = true;
  ApiError? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    final api = context.read<AppState>().api;
    try {
      final earnings = await api.earnings();
      BankAccount? bank;
      try {
        bank = await api.bankAccount();
      } on ApiError catch (e) {
        // Mungesa e llogarisë bankare nuk është gabim: shoferi thjesht s'e ka shtuar ende.
        if (!e.isNotFound) rethrow;
      }
      if (!mounted) return;
      setState(() {
        _earnings = earnings;
        _bank = bank;
        _error = null;
        _loading = false;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;

    if (_loading) {
      return const SafeArea(
        child: Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton()),
      );
    }
    final e = _earnings;
    if (e == null) {
      return SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(K.s5),
          child: KError(
            message: context.tError(_error?.messageKey ?? 'errors.internal'),
            retryLabel: context.t('common.retry'),
            onRetry: _load,
          ),
        ),
      );
    }

    final bank = _bank;
    return SafeArea(
      child: RefreshIndicator(
        onRefresh: _load,
        color: K.brand400,
        backgroundColor: K.surface2,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            Text(
              context.t('driver.earnings'),
              style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: K.text),
            ),
            const SizedBox(height: K.s4),
            KCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    context.t('driver.earnings.today'),
                    style: const TextStyle(
                      fontSize: 13,
                      color: K.muted,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: K.s2),
                  KMoney(e.todayMinor, currency: e.currency, locale: locale, size: 34),
                  const SizedBox(height: K.s2),
                  Text(
                    context.t('driver.earnings.rides', {'n': '${e.ridesToday}'}),
                    style: const TextStyle(fontSize: 12, color: K.muted),
                  ),
                ],
              ),
            ),
            const SizedBox(height: K.s4),
            KCard(
              child: Column(
                children: [
                  KMoneyRow(
                    context.t('driver.earnings.week'),
                    e.weekMinor,
                    currency: e.currency,
                    locale: locale,
                    hint: context.t('driver.earnings.rides', {'n': '${e.ridesWeek}'}),
                  ),
                  KMoneyRow(
                    context.t('driver.earnings.month'),
                    e.monthMinor,
                    currency: e.currency,
                    locale: locale,
                  ),
                  KMoneyRow(
                    context.t('driver.earnings.cash'),
                    e.cashCollectedWeekMinor,
                    currency: e.currency,
                    locale: locale,
                  ),
                  const KMoneyDivider(),
                  KMoneyRow(
                    context.t('driver.earnings.balance'),
                    e.balanceMinor,
                    currency: e.currency,
                    locale: locale,
                    total: true,
                  ),
                ],
              ),
            ),
            const SizedBox(height: K.s3),
            Text(
              context.t('driver.earnings.threshold', {
                'amount': formatMinor(e.nextPayoutMinMinor, currency: e.currency, locale: locale),
              }),
              style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
            ),
            const SizedBox(height: K.s6),
            KSectionHeader(context.t('driver.bank')),
            const SizedBox(height: K.s3),
            KCard(
              onTap: () async {
                await Navigator.of(
                  context,
                ).push<bool>(MaterialPageRoute(builder: (_) => BankAccountScreen(current: bank)));
                await _load();
              },
              child: Row(
                children: [
                  Icon(
                    Icons.account_balance_outlined,
                    size: 20,
                    color: bank == null ? K.warn : K.ok,
                  ),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          bank?.ibanMasked ?? context.t('driver.bank.missing'),
                          style: const TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w600,
                            color: K.text,
                          ),
                        ),
                        if (bank != null)
                          Padding(
                            padding: const EdgeInsets.only(top: 2),
                            child: Text(
                              bank.holderName,
                              style: const TextStyle(fontSize: 12, color: K.muted),
                            ),
                          ),
                      ],
                    ),
                  ),
                  if (bank?.verified == true)
                    KBadge(context.t('driver.bank.verified'), tone: KTone.ok),
                  const SizedBox(width: K.s2),
                  const Icon(Icons.chevron_right, size: 20, color: K.line2),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
