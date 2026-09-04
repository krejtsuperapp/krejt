import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../ride/map_scaffold.dart';

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

/// Ndjekja e porosisë mbi hartë: partneri, adresa e dorëzimit, rruga mes tyre dhe korrieri
/// në kohë reale (kanali `order:{id}`). Kodi gjashtështkronjor shfaqet që klienti ta krahasojë
/// me korrierin, dhe anulimi zhduket sapo kuzhina nis punën (§19).
class OrderTrackingScreen extends StatefulWidget {
  const OrderTrackingScreen({super.key, required this.orderId});

  final String orderId;

  @override
  State<OrderTrackingScreen> createState() => _OrderTrackingScreenState();
}

class _OrderTrackingScreenState extends State<OrderTrackingScreen> {
  static const _pollEvery = Duration(seconds: 6);

  Timer? _timer;
  StreamSubscription<RealtimeEvent>? _live;
  Order? _order;
  LatLng? _courierAt;
  List<MapPoint>? _path;
  ApiError? _error;
  bool _cancelling = false;
  bool _routeAsked = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _poll());
    _timer = Timer.periodic(_pollEvery, (_) => _poll());
    _live = context
        .read<AppState>()
        .realtime
        .subscribe(orderChannel(widget.orderId))
        .listen(_onLive);
  }

  void _onLive(RealtimeEvent e) {
    if (!mounted) return;
    if (e.type == 'courier_location') {
      final lat = e.payload['lat'], lng = e.payload['lng'];
      if (lat is num && lng is num) {
        setState(() => _courierAt = LatLng(lat.toDouble(), lng.toDouble()));
      }
      return;
    }
    unawaited(_poll());
  }

  @override
  void dispose() {
    _timer?.cancel();
    _live?.cancel();
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
      if (!_routeAsked) unawaited(_route(order));
      if (!order.isActive) _timer?.cancel();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = e);
    }
  }

  Future<void> _route(Order order) async {
    final from = order.merchantLocation;
    final to = order.address;
    if (from == null || to == null || order.fulfillment != 'courier') return;
    _routeAsked = true;
    try {
      final r = await context.read<AppState>().api.routePath(from, to);
      if (!mounted) return;
      setState(() => _path = [for (final p in r.points) MapPoint(p.lat, p.lng)]);
    } on ApiError {
      _routeAsked = false;
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

  List<MapMarker> _markers(Order? order) {
    if (order == null) return const [];
    final m = order.merchantLocation;
    final a = order.address;
    final courierMoving =
        order.state == OrderState.courierAssigned || order.state == OrderState.pickedUp;
    return [
      if (m != null) markerOf(m.lat, m.lng, MapMarkerKind.place),
      if (a != null) markerOf(a.lat, a.lng, MapMarkerKind.dropoff),
      if (_courierAt != null && courierMoving)
        markerOf(_courierAt!.lat, _courierAt!.lng, MapMarkerKind.driver),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    final order = _order;
    return MapScaffold.sheet(
      title: context.t('order.title'),
      markers: _markers(order),
      path: _path,
      sheet: (controller) => order == null
          ? ListView(
              controller: controller,
              padding: const EdgeInsets.all(K.s5),
              children: [
                const Center(child: KSheetHandle()),
                _error == null
                    ? KLoading(label: context.t('common.loading'))
                    : KError(
                        message: context.tError(_error!.messageKey),
                        retryLabel: context.t('common.retry'),
                        onRetry: _poll,
                      ),
              ],
            )
          : _content(context, controller, order, locale),
    );
  }

  Widget _content(BuildContext context, ScrollController controller, Order order, String locale) {
    final ready = order.readyAtEstimate;
    final courierMoving =
        order.state == OrderState.courierAssigned || order.state == OrderState.pickedUp;
    return ListView(
      controller: controller,
      padding: const EdgeInsets.fromLTRB(K.s5, 0, K.s5, K.s8),
      children: [
        const Center(child: KSheetHandle()),
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
        if (courierMoving && _courierAt != null) ...[
          const SizedBox(height: K.s3),
          KNeonBanner(
            icon: Icons.two_wheeler_outlined,
            title: context.t('order.courier.live'),
            subtitle: order.addressText ?? '',
          ),
        ],
        const SizedBox(height: K.s4),
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
        const SizedBox(height: K.s4),
        _Steps(order: order),
        const SizedBox(height: K.s4),
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

/// Hapat e porosisë si vijë progresi: përdoruesi e sheh sa larg është dorëzimi pa lexuar tekst.
class _Steps extends StatelessWidget {
  const _Steps({required this.order});

  final Order order;

  static const _flow = [
    OrderState.pendingMerchant,
    OrderState.accepted,
    OrderState.preparing,
    OrderState.ready,
    OrderState.courierAssigned,
    OrderState.pickedUp,
    OrderState.delivered,
  ];

  @override
  Widget build(BuildContext context) {
    final idx = _flow.indexOf(order.state);
    final ended = order.state == OrderState.cancelled || order.state == OrderState.rejected;
    return Row(
      children: [
        for (var i = 0; i < _flow.length; i++)
          Expanded(
            child: Container(
              height: 5,
              margin: EdgeInsets.only(right: i == _flow.length - 1 ? 0 : 4),
              decoration: BoxDecoration(
                color: ended
                    ? K.line2
                    : i <= idx
                    ? K.brand500
                    : K.line2,
                borderRadius: BorderRadius.circular(K.rFull),
                boxShadow: !ended && i == idx
                    ? [BoxShadow(color: K.brand500.withValues(alpha: 0.55), blurRadius: 8)]
                    : null,
              ),
            ),
          ),
      ],
    );
  }
}
