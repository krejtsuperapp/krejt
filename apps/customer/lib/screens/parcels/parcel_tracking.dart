import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../ride/map_scaffold.dart';

String parcelStateKey(ParcelState s) {
  switch (s) {
    case ParcelState.requested:
      return 'parcel.state.requested';
    case ParcelState.courierAssigned:
      return 'parcel.state.courier_assigned';
    case ParcelState.pickedUp:
      return 'parcel.state.picked_up';
    case ParcelState.delivered:
      return 'parcel.state.delivered';
    case ParcelState.cancelled:
      return 'parcel.state.cancelled';
    case ParcelState.noCourier:
      return 'parcel.state.no_courier';
  }
}

/// Ndjekja e pakos mbi hartë: marrja, dorëzimi, rruga dhe korrieri live (kanali `parcel:{id}`).
/// Kodet e marrjes dhe të dorëzimit shfaqen të mëdha — klienti ia thotë korrierit dhe marrësit.
class ParcelTrackingScreen extends StatefulWidget {
  const ParcelTrackingScreen({super.key, required this.parcelId});

  final String parcelId;

  @override
  State<ParcelTrackingScreen> createState() => _ParcelTrackingScreenState();
}

class _ParcelTrackingScreenState extends State<ParcelTrackingScreen> {
  static const _pollEvery = Duration(seconds: 8);

  Timer? _timer;
  StreamSubscription<RealtimeEvent>? _live;
  Parcel? _parcel;
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
        .subscribe(parcelChannel(widget.parcelId))
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
      final parcel = await context.read<AppState>().api.parcel(widget.parcelId);
      if (!mounted) return;
      setState(() {
        _parcel = parcel;
        _error = null;
      });
      if (!_routeAsked) unawaited(_route(parcel));
      if (parcel.isFinished) {
        _timer?.cancel();
        unawaited(context.read<AppState>().refreshHome());
      }
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = e);
    }
  }

  Future<void> _route(Parcel p) async {
    _routeAsked = true;
    try {
      final r = await context.read<AppState>().api.routePath(p.pickup, p.dropoff);
      if (!mounted) return;
      setState(() => _path = [for (final pt in r.points) MapPoint(pt.lat, pt.lng)]);
    } on ApiError {
      _routeAsked = false;
    }
  }

  Future<void> _cancel() async {
    final parcel = _parcel;
    if (parcel == null) return;
    final ok = await confirmKSheet(
      context: context,
      title: context.t('parcel.cancel.confirm'),
      message: context.t('parcel.cancel.body'),
      confirmLabel: context.t('parcel.cancel'),
      cancelLabel: context.t('common.no'),
      destructive: true,
    );
    if (!ok || !mounted) return;
    setState(() => _cancelling = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final updated = await context.read<AppState>().api.cancelParcel(parcel.id);
      if (!mounted) return;
      setState(() => _parcel = updated);
      _timer?.cancel();
      unawaited(context.read<AppState>().refreshHome());
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    } finally {
      if (mounted) setState(() => _cancelling = false);
    }
  }

  List<MapMarker> _markers(Parcel? p) {
    if (p == null) return const [];
    final courierMoving = p.state == ParcelState.courierAssigned || p.state == ParcelState.pickedUp;
    return [
      markerOf(p.pickup.lat, p.pickup.lng, MapMarkerKind.pickup),
      markerOf(p.dropoff.lat, p.dropoff.lng, MapMarkerKind.dropoff),
      if (_courierAt != null && courierMoving)
        markerOf(_courierAt!.lat, _courierAt!.lng, MapMarkerKind.driver),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    final parcel = _parcel;
    return MapScaffold.sheet(
      title: context.t('parcel.title'),
      markers: _markers(parcel),
      path: _path,
      sheet: (controller) => parcel == null
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
          : _content(context, controller, parcel, locale),
    );
  }

  Widget _content(BuildContext context, ScrollController controller, Parcel p, String locale) {
    final courier = p.courier;
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
          context.t(parcelStateKey(p.state)),
          style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: K.text),
        ),
        const SizedBox(height: K.s1),
        Text(
          '${context.t('parcel.size.${p.size}')} · ${p.code}',
          style: const TextStyle(fontSize: 13, color: K.muted),
        ),
        if (p.state == ParcelState.requested) ...[
          const SizedBox(height: K.s4),
          KCard(child: KLoading(label: context.t('ride.searching'))),
        ],
        if (p.state == ParcelState.noCourier) ...[
          const SizedBox(height: K.s4),
          KEmpty(
            title: context.t('parcel.state.no_courier'),
            message: context.t('parcel.no_courier.hint'),
            icon: Icons.search_off,
          ),
        ],
        if (p.isActive) ...[
          const SizedBox(height: K.s4),
          Row(
            children: [
              Expanded(
                child: _CodeCard(
                  label: context.t('parcel.code.pickup'),
                  code: p.pickupCode ?? '••••',
                  hint: context.t('parcel.code.pickup.hint'),
                  done: p.pickedUpAt != null,
                ),
              ),
              const SizedBox(width: K.s3),
              Expanded(
                child: _CodeCard(
                  label: context.t('parcel.code.delivery'),
                  code: p.deliveryCode ?? '••••',
                  hint: context.t('parcel.code.delivery.hint'),
                  done: p.deliveredAt != null,
                ),
              ),
            ],
          ),
        ],
        if (courier != null && p.isActive) ...[
          const SizedBox(height: K.s4),
          KCard(
            child: Row(
              children: [
                Container(
                  width: 46,
                  height: 46,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    color: K.surface3,
                    borderRadius: BorderRadius.circular(K.rFull),
                  ),
                  child: const Icon(Icons.two_wheeler_outlined, color: K.textDim),
                ),
                const SizedBox(width: K.s3),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        courier.name.isEmpty ? context.t('parcel.courier') : courier.name,
                        style: const TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                          color: K.text,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(courier.vehicle, style: const TextStyle(fontSize: 13, color: K.textDim)),
                    ],
                  ),
                ),
                if (courier.vehiclePlate.isNotEmpty)
                  KBadge(courier.vehiclePlate, tone: KTone.brand),
              ],
            ),
          ),
        ],
        const SizedBox(height: K.s4),
        KCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              RouteEnds(pickup: p.pickupAddress ?? '—', dropoff: p.dropoffAddress ?? '—'),
              const SizedBox(height: K.s3),
              KRow(context.t('parcel.recipient'), '${p.recipientName} · ${p.recipientPhone}'),
              KRow(
                context.t('ride.payment'),
                context.t(
                  p.paymentMethod == 'wallet' ? 'ride.payment.wallet' : 'ride.payment.cash',
                ),
              ),
              const KMoneyDivider(),
              KMoneyRow(
                context.t('ride.summary'),
                p.priceMinor,
                currency: p.currency,
                locale: locale,
                total: true,
              ),
            ],
          ),
        ),
        const SizedBox(height: K.s4),
        _Timeline(parcel: p),
        const SizedBox(height: K.s5),
        if (p.canCancel)
          KOutlineButton(
            label: context.t('parcel.cancel'),
            icon: Icons.close,
            danger: true,
            onPressed: _cancelling ? null : _cancel,
          ),
        if (p.isFinished)
          KButton(label: context.t('common.close'), onPressed: () => Navigator.of(context).pop()),
      ],
    );
  }
}

class _CodeCard extends StatelessWidget {
  const _CodeCard({
    required this.label,
    required this.code,
    required this.hint,
    required this.done,
  });

  final String label;
  final String code;
  final String hint;
  final bool done;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(K.s4),
    decoration: BoxDecoration(
      color: K.surface2,
      borderRadius: BorderRadius.circular(K.rMd),
      border: Border.all(color: done ? K.line : K.brand500.withValues(alpha: 0.5)),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                label,
                style: const TextStyle(fontSize: 12, color: K.muted, fontWeight: FontWeight.w600),
              ),
            ),
            if (done) const Icon(Icons.check_circle, size: 16, color: K.ok),
          ],
        ),
        const SizedBox(height: K.s2),
        Text(
          code.split('').join(' '),
          style: TextStyle(
            fontSize: 28,
            fontWeight: FontWeight.w800,
            letterSpacing: 4,
            color: done ? K.muted : K.text,
            fontFeatures: const [FontFeature.tabularFigures()],
          ),
        ),
        const SizedBox(height: K.s1),
        Text(hint, style: const TextStyle(fontSize: 11, color: K.muted, height: 1.3)),
      ],
    ),
  );
}

class _Timeline extends StatelessWidget {
  const _Timeline({required this.parcel});

  final Parcel parcel;

  @override
  Widget build(BuildContext context) {
    final steps = <MapEntry<String, DateTime?>>[
      MapEntry('parcel.timeline.requested', parcel.createdAt),
      MapEntry('parcel.timeline.assigned', parcel.assignedAt),
      MapEntry('parcel.timeline.picked_up', parcel.pickedUpAt),
      MapEntry('parcel.timeline.delivered', parcel.deliveredAt),
    ];
    return KCard(
      child: Column(
        children: [
          for (final step in steps)
            SizedBox(
              height: 34,
              child: Row(
                children: [
                  Icon(
                    step.value == null ? Icons.circle_outlined : Icons.check_circle,
                    size: 18,
                    color: step.value == null ? K.line2 : K.ok,
                  ),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Text(
                      context.t(step.key),
                      style: TextStyle(
                        fontSize: 14,
                        color: step.value == null ? K.muted : K.textDim,
                      ),
                    ),
                  ),
                  if (step.value != null)
                    Text(
                      '${step.value!.hour.toString().padLeft(2, '0')}:'
                      '${step.value!.minute.toString().padLeft(2, '0')}',
                      style: const TextStyle(fontSize: 12, color: K.muted),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
