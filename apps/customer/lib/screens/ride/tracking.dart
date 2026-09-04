import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../home.dart';
import 'chat.dart';
import 'map_scaffold.dart';
import 'safety.dart';
import 'review.dart';

/// Ndjekja e udhëtimit mbi hartë të plotë. Gjendja vjen nga kanali i gjallë (§42): çdo ngjarje
/// e udhëtimit rifreskon gjendjen nga serveri, ndërsa pozicioni i shoferit zbatohet drejtpërdrejt
/// te harta. Pyetja periodike mbetet si rrugë rezervë — pa lidhje, ekrani punon njësoj.
class TrackingScreen extends StatefulWidget {
  const TrackingScreen({super.key, required this.rideId});

  final String rideId;

  @override
  State<TrackingScreen> createState() => _TrackingScreenState();
}

class _TrackingScreenState extends State<TrackingScreen> {
  // Rezervë: kanali i gjallë e mbulon rrjedhën; kjo e kap vetëm rastin kur ai bie.
  static const _pollEvery = Duration(seconds: 15);

  Timer? _timer;
  StreamSubscription<RealtimeEvent>? _live;

  /// Pozicioni i fundit i shoferit nga kanali; më i freskët se ai i profilit të udhëtimit.
  LatLng? _driverAt;
  Ride? _ride;
  List<MapPoint>? _path;
  ApiError? _error;
  bool _cancelling = false;
  bool _reviewShown = false;
  bool _routeAsked = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _poll());
    _timer = Timer.periodic(_pollEvery, (_) => _poll());
    _live = context.read<AppState>().realtime.subscribe(rideChannel(widget.rideId)).listen(_onLive);
  }

  void _onLive(RealtimeEvent e) {
    if (!mounted) return;
    if (e.type == 'driver_location') {
      final lat = e.payload['lat'], lng = e.payload['lng'];
      if (lat is num && lng is num) {
        setState(() => _driverAt = LatLng(lat.toDouble(), lng.toDouble()));
      }
      return;
    }
    // Çdo ngjarje tjetër e udhëtimit: gjendja e plotë merret nga serveri, që modeli të mbetet një.
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
      final ride = await context.read<AppState>().api.ride(widget.rideId);
      if (!mounted) return;
      setState(() {
        _ride = ride;
        _error = null;
      });
      if (!_routeAsked) unawaited(_route(ride));
      if (ride.isFinished) {
        _timer?.cancel();
        unawaited(context.read<AppState>().refreshHome());
        if (ride.state == RideState.completed) await _askForReview(ride);
      }
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = e);
    }
  }

  /// Gjeometria e rrugës merret një herë; nëse mungon, harta tregon vetëm pikat.
  Future<void> _route(Ride ride) async {
    _routeAsked = true;
    try {
      final r = await context.read<AppState>().api.routePath(ride.pickup, ride.dropoff);
      if (!mounted) return;
      setState(() => _path = [for (final p in r.points) MapPoint(p.lat, p.lng)]);
    } on ApiError {
      _routeAsked = false;
    }
  }

  Future<void> _askForReview(Ride ride) async {
    if (_reviewShown || !mounted) return;
    _reviewShown = true;
    await Navigator.of(context)
        .push(MaterialPageRoute<void>(builder: (_) => ReviewScreen(ride: ride)));
  }

  /// Tarifa e anulimit e vendos serveri; ekrani vetëm e thotë sa është para se të pyesë (§18).
  Future<void> _cancel() async {
    final ride = _ride;
    if (ride == null) return;
    final locale = context.read<AppState>().locale;
    final fee = ride.cancellationFeeMinor;
    final ok = await confirmKSheet(
      context: context,
      title: context.t('ride.cancel.confirm'),
      message: fee > 0
          ? context.t('ride.cancel.charged', {'amount': formatMinor(fee, locale: locale)})
          : context.t('ride.cancel.free'),
      confirmLabel: context.t('ride.cancel'),
      cancelLabel: context.t('common.no'),
      destructive: true,
    );
    if (!ok || !mounted) return;
    setState(() => _cancelling = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final updated = await context.read<AppState>().api.cancelRide(ride.id);
      if (!mounted) return;
      setState(() => _ride = updated);
      _timer?.cancel();
      unawaited(context.read<AppState>().refreshHome());
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    } finally {
      if (mounted) setState(() => _cancelling = false);
    }
  }

  Future<void> _showPickupCode() async {
    final ride = _ride;
    if (ride == null) return;
    await showKSheet<void>(
      context: context,
      title: context.t('ride.qr.title'),
      subtitle: context.t('ride.qr.hint'),
      child: _PickupCode(ride: ride),
    );
  }

  /// Shoferi shtohet vetëm kur serveri ka dërguar pozicionin e tij; një shenjë e ngrirë te
  /// pika e marrjes do të thoshte diçka që nuk dihet.
  List<MapMarker> _markers(Ride? ride) {
    if (ride == null) return const [];
    final driverAt = _driverAt ?? ride.driver?.location;
    return [
      markerOf(ride.pickup.lat, ride.pickup.lng, MapMarkerKind.pickup),
      markerOf(ride.dropoff.lat, ride.dropoff.lng, MapMarkerKind.dropoff),
      if (driverAt != null && ride.isActive)
        markerOf(driverAt.lat, driverAt.lng, MapMarkerKind.driver),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final ride = _ride;
    final locale = context.watch<AppState>().locale;
    return MapScaffold.sheet(
      title: context.t('ride.tracking.title'),
      markers: _markers(ride),
      path: _path,
      sheet: (controller) => ride == null
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
          : _content(context, controller, ride, locale),
    );
  }

  Widget _content(BuildContext context, ScrollController controller, Ride ride, String locale) {
    final driver = ride.driver;
    return ListView(
      controller: controller,
      padding: const EdgeInsets.fromLTRB(K.s5, 0, K.s5, K.s8),
      children: [
        const Center(child: KSheetHandle()),
        if (_error?.isOffline == true) ...[
          KOfflineBar(label: context.t('state.offline')),
          const SizedBox(height: K.s3),
        ],
        _StateHeader(ride: ride),
        const SizedBox(height: K.s4),
        if (ride.state == RideState.matching)
          KCard(
            child: Column(
              children: [
                KLoading(label: context.t('ride.searching')),
                const SizedBox(height: K.s2),
                Text(
                  context.t('ride.searching.hint'),
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 13, color: K.muted),
                ),
              ],
            ),
          ),
        if (ride.state == RideState.noDriver)
          KEmpty(
            title: context.t('ride.no_driver'),
            message: context.t('ride.no_driver.hint'),
            icon: Icons.search_off,
          ),
        if (driver != null) ...[
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
                  child: const Icon(Icons.person, color: K.textDim),
                ),
                const SizedBox(width: K.s3),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        driver.name ?? context.t('home.services.ride'),
                        style: const TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                          color: K.text,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(driver.vehicle, style: const TextStyle(fontSize: 13, color: K.textDim)),
                      if (driver.rating != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 2),
                          child: Text(
                            context.t('ride.driver.rating', {
                              'rating': driver.rating!.toStringAsFixed(1),
                            }),
                            style: const TextStyle(fontSize: 12, color: K.muted),
                          ),
                        ),
                    ],
                  ),
                ),
                KBadge(driver.vehiclePlate, tone: KTone.brand),
              ],
            ),
          ),
          const SizedBox(height: K.s3),
          Row(
            children: [
              if (ride.pickupCode != null && ride.state != RideState.inProgress)
                Expanded(
                  child: KOutlineButton(
                    label: context.t('ride.pickup_code'),
                    icon: Icons.qr_code_2,
                    onPressed: _showPickupCode,
                  ),
                ),
              if (ride.pickupCode != null && ride.state != RideState.inProgress)
                const SizedBox(width: K.s3),
              if (ride.chatOpen)
                Expanded(
                  child: KOutlineButton(
                    label: context.t('ride.chat'),
                    icon: Icons.chat_bubble_outline,
                    onPressed: () => Navigator.of(context)
                        .push(MaterialPageRoute<void>(builder: (_) => ChatScreen(rideId: ride.id))),
                  ),
                ),
            ],
          ),
        ],
        const SizedBox(height: K.s5),
        KCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              RouteEnds(pickup: ride.pickupAddress ?? '—', dropoff: ride.dropoffAddress ?? '—'),
              const SizedBox(height: K.s3),
              KRow(
                context.t('ride.payment'),
                context.t(
                  ride.paymentMethod == 'wallet' ? 'ride.payment.wallet' : 'ride.payment.cash',
                ),
              ),
              const KMoneyDivider(),
              KMoneyRow(
                context.t('ride.summary'),
                ride.priceMinor,
                currency: ride.currency,
                locale: locale,
                total: true,
              ),
              if (ride.cancellationFeeMinor > 0 && ride.state == RideState.cancelled)
                KMoneyRow(
                  context.t('ride.cancel'),
                  ride.cancellationFeeMinor,
                  currency: ride.currency,
                  locale: locale,
                ),
            ],
          ),
        ),
        const SizedBox(height: K.s5),
        _Timeline(ride: ride),
        const SizedBox(height: K.s5),
        // Gjatë udhëtimit anulimi zhduket, ndaj pa këtë buton përdoruesi do të mbetej pa asnjë
        // rrugë pikërisht kur i duhet më shumë.
        if (ride.isActive) ...[
          KOutlineButton(
            label: context.t('safety.title'),
            icon: Icons.shield_outlined,
            onPressed: () => showSafetySheet(context, rideId: ride.id),
          ),
          const SizedBox(height: K.s3),
        ],
        // Anulimi lejohet para nisjes; gjatë udhëtimit rruga e vetme është mbështetja (§18).
        if (ride.isActive && ride.state != RideState.inProgress)
          KOutlineButton(
            label: context.t('ride.cancel'),
            icon: Icons.close,
            danger: true,
            onPressed: _cancelling ? null : _cancel,
          ),
        if (ride.isFinished)
          KButton(label: context.t('common.close'), onPressed: () => Navigator.of(context).pop()),
      ],
    );
  }
}

class _StateHeader extends StatelessWidget {
  const _StateHeader({required this.ride});

  final Ride ride;

  @override
  Widget build(BuildContext context) {
    final name = ride.driver?.name;
    final title = ride.state == RideState.assigned && name != null
        ? context.t('ride.assigned', {'name': name})
        : context.t(rideStateKey(ride.state));
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: K.text),
        ),
        const SizedBox(height: K.s1),
        Text(
          context.t(rideCategoryKeyFor(ride)),
          style: const TextStyle(fontSize: 13, color: K.muted),
        ),
      ],
    );
  }
}

String rideCategoryKeyFor(Ride ride) => 'ride.category.${ride.category.name}';

/// Rrjedha e udhëtimit si hapa të kryer, që përdoruesi ta dijë ku ndodhet pa e pyetur shoferin.
class _Timeline extends StatelessWidget {
  const _Timeline({required this.ride});

  final Ride ride;

  @override
  Widget build(BuildContext context) {
    final steps = <MapEntry<String, DateTime?>>[
      MapEntry('ride.timeline.requested', ride.requestedAt),
      MapEntry('ride.timeline.assigned', ride.assignedAt),
      MapEntry('ride.timeline.arrived', ride.arrivedAt),
      MapEntry('ride.timeline.started', ride.startedAt),
      MapEntry('ride.timeline.completed', ride.completedAt),
    ];
    return KCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
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

/// Kodi 4-shifror shfaqet gjithmonë; token-i i QR-së merret nga serveri dhe skadon për 5 minuta,
/// ndaj ekrani tregon sa i mbetet dhe e rimerr kur duhet (§25).
class _PickupCode extends StatefulWidget {
  const _PickupCode({required this.ride});

  final Ride ride;

  @override
  State<_PickupCode> createState() => _PickupCodeState();
}

class _PickupCodeState extends State<_PickupCode> {
  PickupToken? _token;
  String? _error;
  Timer? _ticker;
  int _secondsLeft = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _fetch());
  }

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  Future<void> _fetch() async {
    try {
      final token = await context.read<AppState>().api.pickupQr(widget.ride.id);
      if (!mounted) return;
      setState(() {
        _token = token;
        _error = null;
      });
      _ticker?.cancel();
      _ticker = Timer.periodic(const Duration(seconds: 1), (t) {
        if (!mounted) return t.cancel();
        final left = token.expiresAt.difference(DateTime.now()).inSeconds;
        setState(() => _secondsLeft = left < 0 ? 0 : left);
        if (left <= 0) t.cancel();
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = context.tError(e.messageKey));
    }
  }

  @override
  Widget build(BuildContext context) {
    final code = widget.ride.pickupCode ?? '••••';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Center(
          child: Text(
            code.split('').join(' '),
            style: const TextStyle(
              fontSize: 40,
              fontWeight: FontWeight.w800,
              letterSpacing: 6,
              color: K.text,
              fontFeatures: [FontFeature.tabularFigures()],
            ),
          ),
        ),
        const SizedBox(height: K.s4),
        if (_error != null)
          KError(
            message: _error!,
            retryLabel: context.t('common.retry'),
            onRetry: _fetch,
            icon: Icons.qr_code_2,
          )
        else if (_token != null)
          Center(
            child: Text(
              _secondsLeft > 0
                  ? context.t('ride.qr.expires', {'s': '$_secondsLeft'})
                  : context.t('ride.quote.expired'),
              style: TextStyle(fontSize: 13, color: _secondsLeft > 0 ? K.muted : K.warn),
            ),
          )
        else
          KLoading(label: context.t('common.loading')),
        if (_secondsLeft == 0 && _token != null) ...[
          const SizedBox(height: K.s3),
          KOutlineButton(label: context.t('common.retry'), icon: Icons.refresh, onPressed: _fetch),
        ],
      ],
    );
  }
}
