import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../../state/cart_state.dart';
import '../account/addresses.dart';
import 'order_tracking.dart';

/// Shporta dhe checkout-i. Çdo ndryshim i sasisë kërkon çmim të ri nga serveri:
/// totali që sheh përdoruesi është ai që do të paguhet, jo një përllogaritje lokale (§19).
class CartScreen extends StatefulWidget {
  const CartScreen({super.key});

  @override
  State<CartScreen> createState() => _CartScreenState();
}

class _CartScreenState extends State<CartScreen> {
  OrderQuote? _quote;
  List<Address> _addresses = const [];
  Address? _address;
  String _paymentMethod = 'cash';
  String _fulfillment = 'courier';
  bool _quoting = true;
  bool _placing = false;
  ApiError? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadAddresses();
      _refreshQuote();
    });
  }

  Future<void> _loadAddresses() async {
    try {
      final items = await context.read<AppState>().api.addresses();
      if (!mounted) return;
      setState(() {
        _addresses = items;
        _address ??= items.isEmpty ? null : items.first;
      });
    } on ApiError {
      // Pa adresa mbetet vetëm marrja në vend; ekrani e tregon këtë vetë.
    }
  }

  Future<void> _refreshQuote() async {
    final cart = context.read<CartState>();
    final merchant = cart.merchant;
    if (merchant == null || cart.isEmpty) {
      setState(() {
        _quote = null;
        _quoting = false;
      });
      return;
    }
    setState(() => _quoting = true);
    try {
      final quote = await context.read<AppState>().api.quoteOrder(
        merchantId: merchant.id,
        lines: cart.lines,
        paymentMethod: _paymentMethod,
        fulfillment: _fulfillment,
        addressId: _fulfillment == 'courier' ? _address?.id : null,
      );
      if (!mounted) return;
      setState(() {
        _quote = quote;
        _error = null;
        _quoting = false;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _quoting = false;
      });
    }
  }

  Future<void> _placeOrder() async {
    final cart = context.read<CartState>();
    final merchant = cart.merchant;
    if (merchant == null) return;
    setState(() => _placing = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final order = await context.read<AppState>().api.createOrder(
        merchantId: merchant.id,
        lines: cart.lines,
        paymentMethod: _paymentMethod,
        fulfillment: _fulfillment,
        addressId: _fulfillment == 'courier' ? _address?.id : null,
      );
      if (!mounted) return;
      cart.clear();
      await Navigator.of(context).pushReplacement(
        MaterialPageRoute<void>(builder: (_) => OrderTrackingScreen(orderId: order.id)),
      );
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _placing = false);
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    }
  }

  @override
  Widget build(BuildContext context) {
    final cart = context.watch<CartState>();
    final locale = context.watch<AppState>().locale;
    final quote = _quote;

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(
        title: Text(context.t('cart.title')),
        actions: [
          if (!cart.isEmpty)
            IconButton(
              icon: const Icon(Icons.delete_outline),
              tooltip: context.t('cart.clear'),
              onPressed: () async {
                final ok = await confirmKSheet(
                  context: context,
                  title: context.t('cart.clear.confirm'),
                  message: context.t('cart.empty.hint'),
                  confirmLabel: context.t('cart.clear'),
                  cancelLabel: context.t('common.no'),
                  destructive: true,
                );
                if (ok && context.mounted) {
                  cart.clear();
                  Navigator.of(context).pop();
                }
              },
            ),
        ],
      ),
      body: SafeArea(
        child: cart.isEmpty
            ? Padding(
                padding: const EdgeInsets.all(K.s5),
                child: KEmpty(
                  title: context.t('cart.empty'),
                  message: context.t('cart.empty.hint'),
                  icon: Icons.shopping_bag_outlined,
                ),
              )
            : Column(
                children: [
                  Expanded(child: _list(context, cart, locale)),
                  _footer(context, quote, locale),
                ],
              ),
      ),
    );
  }

  Widget _list(BuildContext context, CartState cart, String locale) => ListView(
    padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s4),
    children: [
      KSectionHeader(context.t('cart.items')),
      const SizedBox(height: K.s3),
      for (var i = 0; i < cart.lines.length; i++)
        Padding(
          padding: const EdgeInsets.only(bottom: K.s2),
          child: _CartRow(
            line: cart.lines[i],
            locale: locale,
            onChanged: (q) {
              cart.setQuantity(i, q);
              _refreshQuote();
            },
          ),
        ),
      const SizedBox(height: K.s5),
      KSectionHeader(context.t('ride.payment')),
      const SizedBox(height: K.s3),
      Row(
        children: [
          Expanded(
            child: _Choice(
              icon: Icons.payments_outlined,
              label: context.t('ride.payment.cash'),
              selected: _paymentMethod == 'cash',
              onTap: () {
                setState(() => _paymentMethod = 'cash');
                _refreshQuote();
              },
            ),
          ),
          const SizedBox(width: K.s3),
          Expanded(
            child: _Choice(
              icon: Icons.account_balance_wallet_outlined,
              label: context.t('ride.payment.wallet'),
              selected: _paymentMethod == 'wallet',
              onTap: () {
                setState(() => _paymentMethod = 'wallet');
                _refreshQuote();
              },
            ),
          ),
        ],
      ),
      const SizedBox(height: K.s5),
      KSectionHeader(context.t('account.addresses')),
      const SizedBox(height: K.s3),
      if (_addresses.isEmpty)
        KEmpty(
          title: context.t('account.address.empty'),
          icon: Icons.place_outlined,
          action: context.t('account.address.add'),
          onAction: () async {
            await Navigator.of(context)
                .push<bool>(MaterialPageRoute(builder: (_) => const AddAddressScreen()));
            await _loadAddresses();
            await _refreshQuote();
          },
        )
      else
        for (final a in _addresses)
          Padding(
            padding: const EdgeInsets.only(bottom: K.s2),
            child: KCard(
              onTap: () {
                setState(() {
                  _address = a;
                  _fulfillment = 'courier';
                });
                _refreshQuote();
              },
              highlight: _address?.id == a.id && _fulfillment == 'courier',
              child: Row(
                children: [
                  Icon(addressIcon(a.label), size: 20, color: K.muted),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Text(
                      '${a.line1}, ${a.city}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 14, color: K.textDim),
                    ),
                  ),
                ],
              ),
            ),
          ),
    ],
  );

  Widget _footer(BuildContext context, OrderQuote? quote, String locale) {
    if (_quoting) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 56, count: 1));
    }
    if (quote == null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.tError(_error?.messageKey ?? 'errors.internal'),
          retryLabel: context.t('common.retry'),
          onRetry: _refreshQuote,
        ),
      );
    }
    final missing = quote.missingForMinimum;
    return Container(
      decoration: const BoxDecoration(
        color: K.surface,
        border: Border(top: BorderSide(color: K.line)),
      ),
      padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, K.s4),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          KMoneyRow(
            context.t('cart.subtotal'),
            quote.itemsTotalMinor,
            currency: quote.currency,
            locale: locale,
          ),
          KMoneyRow(
            context.t('cart.delivery'),
            quote.deliveryFeeMinor,
            currency: quote.currency,
            locale: locale,
          ),
          const KMoneyDivider(),
          KMoneyRow(
            context.t('cart.total'),
            quote.totalMinor,
            currency: quote.currency,
            locale: locale,
            total: true,
          ),
          const SizedBox(height: K.s3),
          if (missing > 0)
            Text(
              context.t('cart.missing', {
                'amount': formatMinor(missing, currency: quote.currency, locale: locale),
              }),
              style: const TextStyle(fontSize: 13, color: K.warn),
            )
          else if (!quote.openNow)
            Text(context.t('food.closed'), style: const TextStyle(fontSize: 13, color: K.warn))
          else
            KButton(
              label: context.t('cart.checkout'),
              busy: _placing,
              onPressed: _placing ? null : _placeOrder,
            ),
        ],
      ),
    );
  }
}

class _CartRow extends StatelessWidget {
  const _CartRow({required this.line, required this.locale, required this.onChanged});

  final CartLine line;
  final String locale;
  final ValueChanged<int> onChanged;

  @override
  Widget build(BuildContext context) => KCard(
    padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
    child: Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                line.product.name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: K.text),
              ),
              if (line.optionIds.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 2),
                  child: Text(
                    '${line.optionIds.length}',
                    style: const TextStyle(fontSize: 12, color: K.muted),
                  ),
                ),
            ],
          ),
        ),
        IconButton(
          icon: const Icon(Icons.remove, size: 18, color: K.muted),
          onPressed: () => onChanged(line.quantity - 1),
        ),
        Text(
          '${line.quantity}',
          style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: K.text),
        ),
        IconButton(
          icon: const Icon(Icons.add, size: 18, color: K.muted),
          onPressed: line.quantity < 50 ? () => onChanged(line.quantity + 1) : null,
        ),
      ],
    ),
  );
}

class _Choice extends StatelessWidget {
  const _Choice({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => KCard(
    onTap: onTap,
    highlight: selected,
    padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
    child: SizedBox(
      height: K.minTap - K.s4,
      child: Row(
        children: [
          Icon(icon, size: 20, color: selected ? K.brand400 : K.muted),
          const SizedBox(width: K.s2),
          Expanded(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: selected ? K.text : K.textDim,
              ),
            ),
          ),
        ],
      ),
    ),
  );
}
