import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Arsyeja pse një shumë mbushjeje nuk pranohet, ose null kur është e vlefshme.
/// Kufijtë vijnë nga serveri; klienti nuk shpik as minimumin as maksimumin (§23).
enum TopUpProblem { empty, tooSmall, tooLarge, notMultiple }

TopUpProblem? checkTopUp(int? amountMinor, WalletLimits limits) {
  if (amountMinor == null || amountMinor <= 0) return TopUpProblem.empty;
  if (amountMinor < limits.minTopupMinor) return TopUpProblem.tooSmall;
  if (amountMinor > limits.maxTopupMinor) return TopUpProblem.tooLarge;
  if (amountMinor % 50 != 0) return TopUpProblem.notMultiple;
  return null;
}

/// Lexon shumën e shkruar me presje ose pikë dhe e kthen në cent.
int? parseAmountMinor(String text) {
  final value = double.tryParse(text.replaceAll(',', '.').trim());
  if (value == null) return null;
  return (value * 100).round();
}

/// Wallet-i: bilanci nga ledger-i, lëvizjet dhe mbushja me kartë.
/// Asnjë shifër nuk llogaritet në pajisje — gjithçka vjen nga serveri (§23).
class WalletScreen extends StatefulWidget {
  const WalletScreen({super.key});

  @override
  State<WalletScreen> createState() => _WalletScreenState();
}

class _WalletScreenState extends State<WalletScreen> {
  WalletOverview? _overview;
  List<WalletTransaction> _transactions = const [];

  /// A mbeti histori më e vjetër. Klienti e mbështet kursorin prej fillimi; ekrani nuk e përdorte,
  /// ndaj historia ndalej te tridhjetë pa asnjë shenjë se kishte më shumë.
  bool _more = true;
  bool _loadingMore = false;
  ApiError? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    final api = context.read<AppState>().api;
    try {
      final results = await Future.wait([api.wallet(), api.walletTransactions(limit: 30)]);
      if (!mounted) return;
      setState(() {
        _overview = results[0] as WalletOverview;
        _transactions = results[1] as List<WalletTransaction>;
        _more = _transactions.length >= 30;
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

  /// Faqja tjetër e historisë, nga rreshti më i vjetër që shihet.
  Future<void> _loadMore() async {
    if (_loadingMore || _transactions.isEmpty) return;
    setState(() => _loadingMore = true);
    try {
      final older = await context.read<AppState>().api.walletTransactions(
        limit: 30,
        before: _transactions.last.createdAt,
      );
      if (!mounted) return;
      setState(() {
        _transactions = [..._transactions, ...older];
        _more = older.length >= 30;
        _loadingMore = false;
      });
    } on ApiError {
      if (mounted) setState(() => _loadingMore = false);
    }
  }

  Future<void> _topUp() async {
    final overview = _overview;
    if (overview == null) return;
    final amount = await showKSheet<int>(
      context: context,
      title: context.t('wallet.topup.amount'),
      subtitle: context.t('wallet.topup.limits', {
        'min': formatMinor(overview.limits.minTopupMinor, locale: context.languageCode),
        'max': formatMinor(overview.limits.maxTopupMinor, locale: context.languageCode),
      }),
      scrollable: true,
      child: _TopUpPicker(limits: overview.limits, locale: context.languageCode),
    );
    if (amount == null || !mounted) return;

    final api = context.read<AppState>().api;
    try {
      final intent = await api.topUp(amount);
      if (!mounted) return;
      await Navigator.of(context)
          .push(MaterialPageRoute<void>(builder: (_) => TopUpStatusScreen(intentId: intent.id)));
      if (!mounted) return;
      await _load();
      if (mounted) await context.read<AppState>().refreshMe();
    } on ApiError catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final locale = state.locale;
    final topUpEnabled = state.config.flag('wallet_topup', fallback: true);

    if (_loading && _overview == null) {
      return const SafeArea(
        child: Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton()),
      );
    }
    if (_overview == null) {
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

    final o = _overview!;
    return SafeArea(
      child: RefreshIndicator(
        onRefresh: _load,
        color: K.brand400,
        backgroundColor: K.surface2,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            Text(
              context.t('wallet.title'),
              style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: K.text),
            ),
            const SizedBox(height: K.s4),
            _BalanceCard(
              overview: o,
              locale: locale,
              topUpEnabled: topUpEnabled,
              onTopUp: _topUp,
              soonLabel: context.t('common.soon'),
            ),
            const SizedBox(height: K.s3),
            Text(
              context.t('wallet.closed_loop'),
              style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
            ),
            const SizedBox(height: K.s6),
            KSectionHeader(context.t('wallet.transactions')),
            const SizedBox(height: K.s3),
            if (_transactions.isEmpty)
              KEmpty(
                title: context.t('wallet.tx.empty'),
                icon: Icons.receipt_long_outlined,
                message: context.t('wallet.closed_loop'),
              )
            else
              for (final t in _transactions)
                Padding(
                  padding: const EdgeInsets.only(bottom: K.s2),
                  child: _TransactionRow(tx: t, locale: locale),
                ),
            if (_more && _transactions.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: K.s3),
                child: _loadingMore
                    ? const KSkeleton(height: 44, count: 1)
                    : KOutlineButton(label: context.t('common.more'), onPressed: _loadMore),
              ),
          ],
        ),
      ),
    );
  }
}

/// Karta e bilancit: e zezë me kufi neon dhe dritë të lehtë — si karta e markës, jo si tabelë.
class _BalanceCard extends StatelessWidget {
  const _BalanceCard({
    required this.overview,
    required this.locale,
    required this.topUpEnabled,
    required this.onTopUp,
    required this.soonLabel,
  });

  final WalletOverview overview;
  final String locale;
  final bool topUpEnabled;
  final VoidCallback onTopUp;
  final String soonLabel;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(K.s5),
    decoration: BoxDecoration(
      borderRadius: BorderRadius.circular(K.rLg),
      border: Border.all(color: K.brand500.withValues(alpha: 0.45)),
      gradient: const LinearGradient(
        begin: Alignment.topLeft,
        end: Alignment.bottomRight,
        colors: [Color(0xFF162A10), Color(0xFF1A1A1A), Color(0xFF0D0D0D)],
      ),
      boxShadow: [BoxShadow(color: K.brand500.withValues(alpha: 0.12), blurRadius: 28)],
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                context.t('wallet.balance').toUpperCase(),
                style: const TextStyle(
                  fontSize: 12,
                  letterSpacing: 1.2,
                  color: K.muted,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
            const KWordmark(size: 16, animate: false, color: K.textDim),
          ],
        ),
        const SizedBox(height: K.s3),
        KMoney(overview.balanceMinor, currency: overview.currency, locale: locale, size: 40),
        const SizedBox(height: K.s5),
        if (topUpEnabled)
          KButton(label: context.t('wallet.topup'), icon: Icons.add, onPressed: onTopUp)
        else
          KBadge(soonLabel, tone: KTone.neutral),
      ],
    ),
  );
}

/// Shumat e gatshme mbulojnë shumicën e rasteve; shuma e lirë mbetet për të tjerat.
/// Kufijtë vijnë nga serveri, ndaj klienti nuk shpik as minimumin as maksimumin.
class _TopUpPicker extends StatefulWidget {
  const _TopUpPicker({required this.limits, required this.locale});

  final WalletLimits limits;
  final String locale;

  @override
  State<_TopUpPicker> createState() => _TopUpPickerState();
}

class _TopUpPickerState extends State<_TopUpPicker> {
  static const _presets = [500, 1000, 2000, 5000];

  final _custom = TextEditingController();
  int? _selected = 1000;
  String? _error;

  @override
  void dispose() {
    _custom.dispose();
    super.dispose();
  }

  int? get _amount => _selected ?? parseAmountMinor(_custom.text);

  String? _validate(int? amount) {
    switch (checkTopUp(amount, widget.limits)) {
      case null:
        return null;
      case TopUpProblem.empty:
        return context.t('errors.validation');
      case TopUpProblem.tooSmall:
        return context.t('wallet.topup.min', {
          'min': formatMinor(widget.limits.minTopupMinor, locale: widget.locale),
        });
      case TopUpProblem.tooLarge:
        return context.t('wallet.topup.max', {
          'max': formatMinor(widget.limits.maxTopupMinor, locale: widget.locale),
        });
      case TopUpProblem.notMultiple:
        return context.t('wallet.topup.step');
    }
  }

  void _submit() {
    final amount = _amount;
    final error = _validate(amount);
    if (error != null) {
      setState(() => _error = error);
      return;
    }
    Navigator.of(context).pop(amount);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Wrap(
          spacing: K.s2,
          runSpacing: K.s2,
          children: [
            for (final p in _presets)
              ChoiceChip(
                label: Text(formatMinor(p, locale: widget.locale)),
                selected: _selected == p,
                onSelected: (_) => setState(() {
                  _selected = p;
                  _custom.clear();
                  _error = null;
                }),
              ),
          ],
        ),
        const SizedBox(height: K.s4),
        KField(
          label: context.t('wallet.topup.custom'),
          controller: _custom,
          hint: '15,00',
          error: _error,
          keyboardType: const TextInputType.numberWithOptions(decimal: true),
          inputFormatters: [FilteringTextInputFormatter.allow(RegExp(r'[0-9.,]'))],
          onChanged: (_) => setState(() {
            _selected = null;
            _error = null;
          }),
        ),
        const SizedBox(height: K.s5),
        KButton(label: context.t('common.continue'), onPressed: _submit),
      ],
    );
  }
}

/// Suksesi i mbushjes nuk vjen nga klienti, por nga webhook-u i ofruesit (§24).
/// Ky ekran pyet serverin për gjendjen e qëllimit derisa të vendoset.
class TopUpStatusScreen extends StatefulWidget {
  const TopUpStatusScreen({super.key, required this.intentId});

  final String intentId;

  @override
  State<TopUpStatusScreen> createState() => _TopUpStatusScreenState();
}

class _TopUpStatusScreenState extends State<TopUpStatusScreen> {
  static const _pollEvery = Duration(seconds: 3);
  static const _giveUpAfter = Duration(minutes: 3);

  Timer? _timer;
  DateTime _startedAt = DateTime.now();
  PaymentIntent? _intent;
  String? _error;

  @override
  void initState() {
    super.initState();
    _startedAt = DateTime.now();
    _poll();
    _timer = Timer.periodic(_pollEvery, (_) => _poll());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _poll() async {
    if (!mounted) return;
    try {
      final intent = await context.read<AppState>().api.paymentIntent(widget.intentId);
      if (!mounted) return;
      setState(() {
        _intent = intent;
        _error = null;
      });
      if (!intent.isPending) _timer?.cancel();
      if (DateTime.now().difference(_startedAt) > _giveUpAfter) _timer?.cancel();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = context.tError(e.messageKey));
    }
  }

  @override
  Widget build(BuildContext context) {
    final intent = _intent;
    final settled = intent?.isSettled == true;
    final failed =
        intent != null &&
        (intent.status == PaymentStatus.failed || intent.status == PaymentStatus.canceled);

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('wallet.topup'))),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(K.s5),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: K.s6),
              Icon(
                settled
                    ? Icons.check_circle_outline
                    : failed
                    ? Icons.error_outline
                    : Icons.hourglass_bottom,
                size: 56,
                color: settled
                    ? K.ok
                    : failed
                    ? K.danger
                    : K.brand400,
              ),
              const SizedBox(height: K.s4),
              Text(
                context.t(
                  settled
                      ? 'wallet.topup.success'
                      : failed
                      ? 'wallet.topup.failed'
                      : 'wallet.topup.waiting',
                ),
                textAlign: TextAlign.center,
                style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: K.text),
              ),
              const SizedBox(height: K.s2),
              Text(
                _error ?? context.t('wallet.topup.waiting.hint'),
                textAlign: TextAlign.center,
                style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.45),
              ),
              if (intent != null) ...[
                const SizedBox(height: K.s5),
                KCard(
                  child: KMoneyRow(
                    context.t('wallet.topup'),
                    intent.amountMinor,
                    currency: intent.currency,
                    locale: context.languageCode,
                    total: true,
                  ),
                ),
              ],
              const Spacer(),
              KButton(
                label: context.t('common.close'),
                onPressed: () => Navigator.of(context).pop(),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _TransactionRow extends StatelessWidget {
  const _TransactionRow({required this.tx, required this.locale});

  final WalletTransaction tx;
  final String locale;

  static const _kinds = {
    'topup': 'wallet.tx.topup',
    'ride': 'wallet.tx.ride',
    'order': 'wallet.tx.order',
    'refund': 'wallet.tx.refund',
    'fee': 'wallet.tx.fee',
  };

  static const _icons = {
    'topup': Icons.add_card_outlined,
    'ride': Icons.directions_car_outlined,
    'order': Icons.lunch_dining_outlined,
    'refund': Icons.undo,
    'fee': Icons.receipt_outlined,
  };

  @override
  Widget build(BuildContext context) {
    final label = context.t(_kinds[tx.kind] ?? 'wallet.tx.other');
    final date = '${tx.createdAt.day}.${tx.createdAt.month}.${tx.createdAt.year}';
    return KCard(
      padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: tx.isCredit ? K.okBg : K.surface2,
              borderRadius: BorderRadius.circular(K.rSm),
            ),
            child: Icon(
              _icons[tx.kind] ?? Icons.swap_horiz,
              size: 18,
              color: tx.isCredit ? K.ok : K.textDim,
            ),
          ),
          const SizedBox(width: K.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  label,
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: K.text),
                ),
                const SizedBox(height: 2),
                Text(date, style: const TextStyle(fontSize: 12, color: K.muted)),
              ],
            ),
          ),
          KMoney(
            tx.amountMinor,
            currency: tx.currency,
            locale: locale,
            size: 16,
            signed: true,
            color: tx.isCredit ? K.ok : K.text,
          ),
        ],
      ),
    );
  }
}
