import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import 'food/order_tracking.dart';
import 'food/reorder.dart';
import 'home.dart' show rideStateKey;
import 'parcels/parcel_tracking.dart';
import 'ride/tracking.dart';
import 'services/request_tracking.dart';

/// Aktiviteti: gjithçka që klienti ka bërë, në një listë të vetme sipas kohës — udhëtime,
/// porosi, pako dhe shërbime. Pa këtë ekran, një porosi e djeshme nuk gjendej askund: ballina
/// tregon vetëm atë që është në vazhdim dhe udhëtimet e fundit.
///
/// Katër burimet lexohen paralelisht dhe secili dështim mbahet veç: nëse porositë bien, udhëtimet
/// shfaqen prapë. Gabimi tregohet vetëm kur nuk mbetet asnjë burim.
class ActivityScreen extends StatefulWidget {
  const ActivityScreen({super.key});

  @override
  State<ActivityScreen> createState() => _ActivityScreenState();
}

enum ActivityKind { ride, order, parcel, service }

/// Një rresht i listës, i shkëputur nga modeli që e prodhoi: renditja dhe filtri punojnë njësoj
/// për të katër shërbimet, dhe ekrani nuk di gjë për ndryshimet mes tyre.
class ActivityEntry {
  const ActivityEntry({
    required this.kind,
    required this.at,
    required this.title,
    required this.stateKey,
    required this.cancelled,
    required this.open,
    this.amountMinor,
    this.reorder,
  });

  final ActivityKind kind;
  final DateTime at;
  final String title;
  final String stateKey;
  final bool cancelled;
  final int? amountMinor;

  /// Ekrani që hapet me prekje; ndërtohet vonë, që lista të mos mbajë katër ekrane në kujtesë.
  final Widget Function() open;

  /// Vetëm porositë e dorëzuara: rindërton shportën nga menuja e sotme. Null do të thotë se
  /// rreshti nuk e mban dot këtë veprim — një udhëtim nuk "riporositet".
  final Future<void> Function(BuildContext)? reorder;
}

/// Bashkon të katër historitë në një listë të vetme, nga më e reja te më e vjetra.
/// I ndarë nga ekrani me qëllim: renditja dhe përkthimi i gjendjeve testohen pa hartë, pa rrjet
/// dhe pa Flutter.
List<ActivityEntry> mergeActivity({
  required List<Ride> rides,
  required List<Order> orders,
  required List<Parcel> parcels,
  required List<ServiceRequest> services,
  String locale = 'sq',
}) {
  final out = <ActivityEntry>[
    for (final r in rides)
      ActivityEntry(
        kind: ActivityKind.ride,
        at: r.requestedAt,
        title: r.dropoffAddress ?? formatDistance(r.distanceM, locale: locale),
        stateKey: rideStateKey(r.state),
        cancelled: r.state == RideState.cancelled,
        amountMinor: r.priceMinor,
        open: () => TrackingScreen(rideId: r.id),
      ),
    for (final o in orders)
      ActivityEntry(
        kind: ActivityKind.order,
        at: o.createdAt,
        title: o.merchantName,
        stateKey: orderStateKey(o.state),
        cancelled: o.state == OrderState.cancelled || o.state == OrderState.rejected,
        amountMinor: o.totalMinor,
        open: () => OrderTrackingScreen(orderId: o.id),
        reorder: o.state == OrderState.delivered && o.items.isNotEmpty
            ? (context) => reorderOrder(context, o)
            : null,
      ),
    for (final p in parcels)
      ActivityEntry(
        kind: ActivityKind.parcel,
        at: p.createdAt,
        title: p.dropoffAddress ?? p.recipientName,
        stateKey: parcelStateKey(p.state),
        cancelled: p.state == ParcelState.cancelled,
        amountMinor: p.priceMinor,
        open: () => ParcelTrackingScreen(parcelId: p.id),
      ),
    for (final s in services)
      ActivityEntry(
        kind: ActivityKind.service,
        at: s.createdAt,
        title: s.title,
        stateKey: serviceStateKey(s.state),
        cancelled: s.state == ServiceState.cancelled,
        amountMinor: s.priceMinor,
        open: () => ServiceTrackingScreen(requestId: s.id),
      ),
  ];
  out.sort((a, b) => b.at.compareTo(a.at));
  return out;
}

IconData activityIcon(ActivityKind k) {
  switch (k) {
    case ActivityKind.ride:
      return Icons.directions_car_outlined;
    case ActivityKind.order:
      return Icons.restaurant_outlined;
    case ActivityKind.parcel:
      return Icons.local_shipping_outlined;
    case ActivityKind.service:
      return Icons.handyman_outlined;
  }
}

/// Filtrat përdorin të njëjtat emra si pllakat e ballinës; një shërbim nuk quhet dy emra.
String activityLabelKey(ActivityKind k) {
  switch (k) {
    case ActivityKind.ride:
      return 'home.services.ride';
    case ActivityKind.order:
      return 'home.services.food';
    case ActivityKind.parcel:
      return 'home.services.courier';
    case ActivityKind.service:
      return 'home.services.services';
  }
}

class _ActivityScreenState extends State<ActivityScreen> {
  List<ActivityEntry> _all = const [];
  ActivityKind? _filter;
  ApiError? _error;
  bool _loading = true;
  bool _loadingMore = false;

  /// A mbeti diçka më e vjetër. Nis e vërtetë dhe fiket kur asnjë burim nuk kthen më asgjë.
  bool _more = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  static const _pageSize = 30;

  /// Të katër burimet faqosen me të njëjtin kursor: koha e rreshtit më të vjetër që shihet.
  /// Meqë secili burim është i renditur sipas kohës, "para kësaj kohe" e vazhdon secilin aty ku
  /// mbeti, pa dublikate dhe pa mbajtur katër kursorë të veçantë.
  Future<void> _load({DateTime? before}) async {
    final more = before != null;
    if (mounted) setState(() => more ? _loadingMore = true : _loading = true);
    final state = context.read<AppState>();
    final api = state.api;
    final cfg = state.config;
    ApiError? failure;

    // Secili burim kthen listën e vet ose bosh kur dështon: një shërbim i fikur nuk duhet ta
    // lërë ekranin bosh, dhe një shërbim që bie nuk duhet t'i fshehë tre të tjerët.
    Future<List<T>> guard<T>(bool enabled, Future<List<T>> Function() call) async {
      if (!enabled) return const [];
      try {
        return await call();
      } on ApiError catch (e) {
        failure ??= e;
        return const [];
      }
    }

    final rides = guard(
      cfg.flag('rides', fallback: true),
      () => api.rideHistory(limit: _pageSize, before: before),
    );
    final orders = guard(
      cfg.flag('food'),
      () => api.orderHistory(limit: _pageSize, before: before),
    );
    final parcels = guard(
      cfg.flag('parcels', fallback: true),
      () => api.parcelHistory(limit: _pageSize, before: before),
    );
    final services = guard(
      cfg.flag('services', fallback: true),
      () => api.serviceRequests(limit: _pageSize, before: before),
    );
    final entries = mergeActivity(
      rides: await rides,
      orders: await orders,
      parcels: await parcels,
      services: await services,
      locale: state.locale,
    );
    if (!mounted) return;

    setState(() {
      _all = more ? [..._all, ...entries] : entries;
      _loading = false;
      _loadingMore = false;
      // Asnjë rresht më i vjetër do të thotë se lista mbaroi vërtet.
      _more = entries.isNotEmpty;
      // Gabimi shfaqet vetëm kur nuk mbeti asgjë për të treguar; ndryshe lista flet vetë.
      _error = _all.isEmpty ? failure : null;
      if (_filter != null && !_all.any((e) => e.kind == _filter)) _filter = null;
    });
  }

  List<ActivityEntry> get _visible =>
      _filter == null ? _all : _all.where((e) => e.kind == _filter).toList();

  Future<void> _open(ActivityEntry e) async {
    await Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => e.open()));
    if (mounted) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    final visible = _visible;
    // Vetëm shërbimet që kanë vërtet diçka në histori marrin një filtër: një filtër që kthen
    // gjithmonë listë bosh do të ishte një buton i vdekur.
    final kinds = ActivityKind.values.where((k) => _all.any((e) => e.kind == k)).toList();

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('activity.title'))),
      body: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (kinds.length > 1)
              SizedBox(
                height: 52,
                child: ListView(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: K.s5, vertical: K.s2),
                  children: [
                    _Filter(
                      label: context.t('activity.all'),
                      selected: _filter == null,
                      onTap: () => setState(() => _filter = null),
                    ),
                    for (final k in kinds) ...[
                      const SizedBox(width: K.s2),
                      _Filter(
                        label: context.t(activityLabelKey(k)),
                        selected: _filter == k,
                        onTap: () => setState(() => _filter = k),
                      ),
                    ],
                  ],
                ),
              ),
            Expanded(
              child: _loading && _all.isEmpty
                  ? const Padding(
                      padding: EdgeInsets.all(K.s5),
                      child: KSkeleton(height: 64, count: 5),
                    )
                  : visible.isEmpty
                  ? Padding(
                      padding: const EdgeInsets.all(K.s5),
                      child: _error != null
                          ? KError(
                              message: context.tError(_error!.messageKey),
                              retryLabel: context.t('common.retry'),
                              onRetry: _load,
                            )
                          : KEmpty(
                              title: context.t('activity.empty'),
                              message: context.t('activity.empty.hint'),
                              icon: Icons.history,
                            ),
                    )
                  : RefreshIndicator(
                      onRefresh: () => _load(),
                      color: K.brand400,
                      backgroundColor: K.surface2,
                      child: ListView.builder(
                        padding: const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, K.s8),
                        // Rreshti i fundit është butoni "më shumë", kur ka ende çka të sillet.
                        itemCount: visible.length + (_more ? 1 : 0),
                        itemBuilder: (_, i) {
                          if (i == visible.length) {
                            return Padding(
                              padding: const EdgeInsets.only(top: K.s3),
                              child: _loadingMore
                                  ? const KSkeleton(height: 44, count: 1)
                                  : KOutlineButton(
                                      label: context.t('common.more'),
                                      onPressed: _all.isEmpty
                                          ? null
                                          : () => _load(before: _all.last.at),
                                    ),
                            );
                          }
                          return Padding(
                            padding: const EdgeInsets.only(bottom: K.s2),
                            child: _Row(
                              entry: visible[i],
                              locale: locale,
                              onTap: () => _open(visible[i]),
                            ),
                          );
                        },
                      ),
                    ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Filter extends StatelessWidget {
  const _Filter({required this.label, required this.selected, required this.onTap});

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Semantics(
    button: true,
    selected: selected,
    child: Material(
      color: selected ? K.brand500.withValues(alpha: 0.14) : K.surface,
      borderRadius: BorderRadius.circular(K.rFull),
      child: InkWell(
        borderRadius: BorderRadius.circular(K.rFull),
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: K.s4),
          alignment: Alignment.center,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(K.rFull),
            border: Border.all(color: selected ? K.brand500 : K.line2),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: selected ? K.brand400 : K.textDim,
            ),
          ),
        ),
      ),
    ),
  );
}

class _Row extends StatelessWidget {
  const _Row({required this.entry, required this.locale, required this.onTap});

  final ActivityEntry entry;
  final String locale;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final amount = entry.amountMinor;
    return KCard(
      onTap: onTap,
      padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: K.surface2,
              borderRadius: BorderRadius.circular(K.rSm),
              border: Border.all(color: K.line),
            ),
            child: Icon(activityIcon(entry.kind), size: 18, color: K.textDim),
          ),
          const SizedBox(width: K.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  entry.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: K.text),
                ),
                const SizedBox(height: 2),
                Text(
                  '${context.t(entry.stateKey)} · ${_when(entry.at)}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12, color: K.muted),
                ),
              ],
            ),
          ),
          if (amount != null) ...[
            const SizedBox(width: K.s3),
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                KMoney(amount, locale: locale, size: 16, strikethrough: entry.cancelled),
                if (entry.reorder != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: InkWell(
                      onTap: () => entry.reorder!(context),
                      child: Text(
                        context.t('food.reorder'),
                        style: const TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w700,
                          color: K.brand400,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ],
        ],
      ),
    );
  }

  /// Data e shkurtër; ora nuk shtohet, sepse te historia rëndon dita, jo minuta.
  static String _when(DateTime at) => '${at.day}.${at.month}.${at.year}';
}
