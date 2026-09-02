import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

String orderStateKey(OrderState s) {
  switch (s) {
    case OrderState.pendingMerchant:
      return 'order.state.pending_merchant';
    case OrderState.accepted:
      return 'order.state.accepted';
    case OrderState.preparing:
      return 'order.state.preparing';
    case OrderState.ready:
      return 'order.state.ready';
    case OrderState.courierAssigned:
      return 'order.state.courier_assigned';
    case OrderState.pickedUp:
      return 'order.state.picked_up';
    case OrderState.delivered:
      return 'order.state.delivered';
    case OrderState.rejected:
      return 'order.state.rejected';
    case OrderState.cancelled:
      return 'order.state.cancelled';
  }
}

/// Ndjekja e porosisë. Kodi gjashtështkronjor shfaqet që klienti ta krahasojë me korrierin,
/// dhe anulimi zhduket sapo kuzhina nis punën (§19).
class OrderTrackingScreen extends StatefulWidget {
  const OrderTrackingScreen({super.key, required this.orderId});

  final String orderId;

  @override
  State<OrderTrackingScreen> createState() => _OrderTrackingScreenState();
}

class _OrderTrackingScreenState extends State<OrderTrackingScreen> {
  static const _pollEvery = Duration(seconds: 6);

  Timer? _timer;
  Order? _order;
  ApiError? _error;
  bool _cancelling = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _poll());
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
      final order = await context.read<AppState>().api.order(widget.orderId);
      if (!mounted) return;
      setState(() {
        _order = order;
        _error = null;
      });
      if (!order.isActive) _timer?.cancel();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = e);
    }
  }

  Future<void> _cancel() async {
    final order = _order;
    if (order == null) return;
    final ok = await confirmKSheet(
      context: context,
      title: context.t('order.cancel.confirm'),
      message: context.t('order.cancel.body'),
      confirmLabel: context.t('order.cancel'),
      cancelLabel: context.t('common.no'),
      destructive: true,
    );
    if (!ok || !mounted) return;
    setState(() => _cancelling = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final updated = await context.read<AppState>().api.cancelOrder(order.id);
      if (!mounted) return;
      setState(() => _order = updated);
      _timer?.cancel();
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    } finally {
      if (mounted) setState(() => _cancelling = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    final order = _order;

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('order.title'))),
      body: SafeArea(
        child: order == null
            ? Padding(
                padding: const EdgeInsets.all(K.s5),
                child: _error == null
                    ? KLoading(label: context.t('common.loading'))
                    : KError(
                        message: context.tError(_error!.messageKey),
                        retryLabel: context.t('common.retry'),
                        onRetry: _poll,
                      ),
              )
            : _content(context, order, locale),
      ),
    );
  }

  Widget _content(BuildContext context, Order order, String locale) {
    final ready = order.readyAtEstimate;
    return ListView(
      padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
      children: [
        if (_error?.isOffline == true) ...[
          KOfflineBar(label: context.t('state.offline')),
          const SizedBox(height: K.s3),
        ],
        Text(
          context.t(orderStateKey(order.state)),
          style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: K.text),
        ),
        const SizedBox(height: K.s1),
        Text(order.merchantName, style: const TextStyle(fontSize: 13, color: K.muted)),
        const SizedBox(height: K.s5),
        KCard(
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      context.t('order.code'),
                      style: const TextStyle(fontSize: 12, color: K.muted),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      order.code,
                      style: const TextStyle(
                        fontSize: 26,
                        fontWeight: FontWeight.w800,
                        letterSpacing: 4,
                        color: K.text,
                      ),
                    ),
                  ],
                ),
              ),
              if (ready != null && order.isActive)
                Text(
                  context.t('order.ready_at', {
                    'time':
                        '${ready.hour.toString().padLeft(2, '0')}:'
                        '${ready.minute.toString().padLeft(2, '0')}',
                  }),
                  style: const TextStyle(fontSize: 13, color: K.textDim),
                ),
            ],
          ),
        ),
        const SizedBox(height: K.s5),
        KCard(
          child: Column(
            children: [
              for (final item in order.items)
                KMoneyRow(
                  '${item.quantity} × ${item.name}',
                  item.totalMinor,
                  currency: order.currency,
                  locale: locale,
                  hint: item.options.isEmpty ? null : item.options.join(', '),
                ),
              const KMoneyDivider(),
              KMoneyRow(
                context.t('cart.subtotal'),
                order.itemsTotalMinor,
                currency: order.currency,
                locale: locale,
              ),
              KMoneyRow(
                context.t('cart.delivery'),
                order.deliveryFeeMinor,
                currency: order.currency,
                locale: locale,
              ),
              KMoneyRow(
                context.t('cart.total'),
                order.totalMinor,
                currency: order.currency,
                locale: locale,
                total: true,
              ),
            ],
          ),
        ),
        const SizedBox(height: K.s5),
        if (order.canCancel)
          KOutlineButton(
            label: context.t('order.cancel'),
            icon: Icons.close,
            danger: true,
            onPressed: _cancelling ? null : _cancel,
          )
        else if (!order.isActive)
          KButton(label: context.t('common.close'), onPressed: () => Navigator.of(context).pop()),
      ],
    );
  }
}
