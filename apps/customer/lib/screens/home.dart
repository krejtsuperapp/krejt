import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../services/location.dart';
import '../state/app_state.dart';
import 'food/discover.dart';
import 'food/menu.dart';
import 'food/order_tracking.dart';
import 'parcels/new_parcel.dart';
import 'parcels/parcel_tracking.dart';
import 'ride/destination.dart';
import 'ride/tracking.dart';
import 'wallet.dart';

/// Çelësi i përkthimit për një gjendje udhëtimi, i mbajtur në një vend të vetëm
/// që asnjë ekran të mos shpikë etiketat e veta.
String rideStateKey(RideState s) {
  switch (s) {
    case RideState.matching:
      return 'ride.state.matching';
    case RideState.assigned:
      return 'ride.state.assigned';
    case RideState.arrived:
      return 'ride.state.arrived';
    case RideState.inProgress:
      return 'ride.state.in_progress';
    case RideState.completed:
      return 'ride.state.completed';
    case RideState.cancelled:
      return 'ride.state.cancelled';
    case RideState.noDriver:
      return 'ride.state.no_driver';
  }
}

/// Ballina sipas markës: përshëndetja, kërkimi, slide-i i vendeve të hapura afër, gjashtë
/// shërbimet, ajo që është në rrjedhë (udhëtim/porosi), vendet afër dhe historiku i shkurtër.
/// Slide-i dhe lista "afër teje" mbushen vetëm me vende të vërteta nga serveri; pa vendndodhje
/// ose pa vende, ato seksione thjesht nuk shfaqen — asnjë ofertë e shpikur (§55).
class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  static const _location = LocationService();

  List<Merchant> _nearby = const [];
  bool _nearbyLoaded = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadNearby());
  }

  /// Vendet afër: një kërkesë e vetme pas hapjes dhe në çdo rifreskim me tërheqje.
  Future<void> _loadNearby() async {
    if (!mounted) return;
    if (!context.read<AppState>().config.flag('food')) return;
    final position = await _location.current();
    if (!mounted || !position.isOk) {
      if (mounted) setState(() => _nearbyLoaded = true);
      return;
    }
    try {
      final items = await context.read<AppState>().api.merchants(
        lat: position.point!.lat,
        lng: position.point!.lng,
        limit: 12,
      );
      if (!mounted) return;
      setState(() {
        _nearby = items;
        _nearbyLoaded = true;
      });
    } on ApiError {
      if (mounted) setState(() => _nearbyLoaded = true);
    }
  }

  Future<void> _refresh() async {
    await Future.wait([context.read<AppState>().refreshHome(), _loadNearby()]);
  }

  void _open(Widget screen) {
    Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => screen));
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final me = state.me;
    final locale = state.locale;
    final active = state.activeRide;
    final activeOrder = state.activeOrder;
    final activeParcel = state.activeParcel;
    final past = state.recentRides.where((r) => r.isFinished).take(4).toList();
    final open = _nearby.where((m) => m.canOrder).toList();
    final foodOn = state.config.flag('food');

    return SafeArea(
      child: RefreshIndicator(
        onRefresh: _refresh,
        color: K.brand500,
        backgroundColor: K.surface2,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, K.s8),
          children: [
            _Header(name: me?.displayName ?? '—', city: open.isNotEmpty ? open.first.city : null),
            const SizedBox(height: K.s4),
            KSearchBar(
              hint: context.t('home.search'),
              onTap: () => _open(
                active == null ? const DestinationScreen() : TrackingScreen(rideId: active.id),
              ),
            ),
            const SizedBox(height: K.s4),
            KHeroCarousel(slides: _slides(context, open, locale)),
            const SizedBox(height: K.s4),
            _ServicesGrid(
              rideReady: state.config.flag('rides', fallback: true),
              foodReady: foodOn,
              marketReady: state.config.flag('market', fallback: true),
              courierReady: state.config.flag('parcels', fallback: true),
              onRide: () => _open(
                active == null ? const DestinationScreen() : TrackingScreen(rideId: active.id),
              ),
              onFood: () => _open(const DiscoverScreen()),
              onMarket: () => _open(const DiscoverScreen(mode: DiscoverMode.market)),
              onCourier: () => _open(
                activeParcel == null
                    ? const NewParcelScreen()
                    : ParcelTrackingScreen(parcelId: activeParcel.id),
              ),
              onPayments: () => _open(const WalletScreen()),
            ),
            if (active != null) ...[
              const SizedBox(height: K.s4),
              KNeonBanner(
                icon: Icons.directions_car_outlined,
                title: context.t('home.active.ride'),
                subtitle: _rideSubtitle(context, active),
                onTap: () => _open(TrackingScreen(rideId: active.id)),
              ),
            ],
            if (activeParcel != null) ...[
              const SizedBox(height: K.s4),
              KNeonBanner(
                icon: Icons.inventory_2_outlined,
                title: context.t('parcel.active'),
                subtitle: context.t(parcelStateKey(activeParcel.state)),
                onTap: () => _open(ParcelTrackingScreen(parcelId: activeParcel.id)),
              ),
            ],
            if (activeOrder != null) ...[
              const SizedBox(height: K.s4),
              KNeonBanner(
                icon: Icons.lunch_dining_outlined,
                title: context.t('home.active.order'),
                subtitle: context.t(orderStateKey(activeOrder.state)),
                onTap: () => _open(OrderTrackingScreen(orderId: activeOrder.id)),
              ),
            ],
            if (foodOn && (_nearby.isNotEmpty || !_nearbyLoaded)) ...[
              KSectionHeader(
                context.t('home.nearby'),
                action: context.t('home.see_all'),
                onAction: () => _open(const DiscoverScreen()),
              ),
              SizedBox(
                height: 212,
                child: !_nearbyLoaded
                    ? const KSkeleton(height: 200, count: 1)
                    : ListView.separated(
                        scrollDirection: Axis.horizontal,
                        clipBehavior: Clip.none,
                        itemCount: _nearby.length,
                        separatorBuilder: (_, _) => const SizedBox(width: K.s3),
                        itemBuilder: (_, i) => _merchantCard(context, _nearby[i], locale),
                      ),
              ),
            ],
            KSectionHeader(context.t('home.recent')),
            if (past.isEmpty)
              KEmpty(
                title: context.t('home.rides.empty'),
                message: context.t('home.rides.empty.hint'),
                icon: Icons.route_outlined,
              )
            else
              for (final r in past)
                Padding(
                  padding: const EdgeInsets.only(bottom: K.s2),
                  child: _RideRow(ride: r, locale: locale),
                ),
          ],
        ),
      ),
    );
  }

  String _rideSubtitle(BuildContext context, Ride ride) {
    final d = ride.driver;
    if (d == null) return context.t(rideStateKey(ride.state));
    return '${context.t(rideStateKey(ride.state))} · ${d.vehicle} · ${d.vehiclePlate}';
  }

  /// Slide-et: vendet e hapura me kopertinë (deri në tre), ndryshe një slide i markës.
  List<KHeroSlide> _slides(BuildContext context, List<Merchant> open, String locale) {
    final withCover = open.where((m) => m.coverUrl != null).take(3).toList();
    if (withCover.isEmpty) {
      // Pa foto nga partnerët, karuseli tregon vetë shërbimet me fotot e paketuara — asnjë
      // shifër dhe asnjë ofertë e shpikur, vetëm ajo që aplikacioni bën vërtet.
      final active = context.read<AppState>().activeRide;
      return [
        KHeroSlide(
          tag: 'KREJT',
          title: context.t('onboarding.s1.title'),
          assetImage: 'assets/onboarding/01.jpg',
          actionLabel: context.t('home.services.ride'),
          onTap: () =>
              _open(active == null ? const DestinationScreen() : TrackingScreen(rideId: active.id)),
        ),
        KHeroSlide(
          tag: 'KREJT',
          title: context.t('onboarding.s2.title'),
          assetImage: 'assets/onboarding/02.jpg',
          actionLabel: context.t('home.services.food'),
          onTap: () => _open(const DiscoverScreen()),
        ),
        KHeroSlide(
          tag: 'KREJT',
          title: context.t('onboarding.s4.title'),
          assetImage: 'assets/onboarding/04.jpg',
          actionLabel: context.t('home.services.courier'),
          onTap: () => _open(const NewParcelScreen()),
        ),
      ];
    }
    return [
      for (final m in withCover)
        KHeroSlide(
          tag: '${context.t('food.open')} · ${formatDistance(m.distanceM, locale: locale)}',
          title: m.name,
          imageUrl: m.coverUrl,
          actionLabel: context.t('home.order'),
          onTap: () => _open(MenuScreen(merchant: m)),
        ),
    ];
  }

  Widget _merchantCard(BuildContext context, Merchant m, String locale) => KMerchantCard(
    name: m.name,
    subtitle: [
      context.t(merchantTypeKey(m.type)),
      if (m.cuisines.isNotEmpty) m.cuisines.first,
      formatDistance(m.distanceM, locale: locale),
    ].join(' · '),
    imageUrl: m.coverUrl ?? m.logoUrl,
    rating: m.rating?.toStringAsFixed(1),
    dimmed: !m.canOrder,
    chips: [
      if (!m.canOrder)
        KChip(context.t('food.closed'))
      else ...[
        KChip('${m.prepTimeMin} min'),
        KChip(
          m.deliveryFeeMinor == 0
              ? context.t('food.delivery_fee', {'amount': formatMinor(0, locale: locale)})
              : formatMinor(m.deliveryFeeMinor, locale: locale),
          neon: m.deliveryFeeMinor == 0,
        ),
      ],
    ],
    onTap: () => _open(MenuScreen(merchant: m)),
  );
}

class _Header extends StatelessWidget {
  const _Header({required this.name, this.city});

  final String name;
  final String? city;

  @override
  Widget build(BuildContext context) => Row(
    crossAxisAlignment: CrossAxisAlignment.center,
    children: [
      Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(context.t('home.welcome'), style: const TextStyle(fontSize: 13, color: K.muted)),
            const SizedBox(height: 2),
            Text(
              name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontSize: 22,
                fontWeight: FontWeight.w800,
                letterSpacing: -0.4,
                color: K.text,
              ),
            ),
          ],
        ),
      ),
      if (city != null) KPill(city!, icon: Icons.location_on_outlined, neon: true),
    ],
  );
}

class _ServicesGrid extends StatelessWidget {
  const _ServicesGrid({
    required this.rideReady,
    required this.foodReady,
    required this.marketReady,
    required this.courierReady,
    required this.onRide,
    required this.onFood,
    required this.onMarket,
    required this.onCourier,
    required this.onPayments,
  });

  final bool rideReady;
  final bool foodReady;
  final bool marketReady;
  final bool courierReady;
  final VoidCallback onRide;
  final VoidCallback onFood;
  final VoidCallback onMarket;
  final VoidCallback onCourier;
  final VoidCallback onPayments;

  @override
  Widget build(BuildContext context) {
    final soon = context.t('common.soon');
    final tiles = [
      KServiceTile(
        icon: Icons.directions_car_outlined,
        label: context.t('home.services.ride'),
        ready: rideReady,
        soonLabel: soon,
        onTap: onRide,
      ),
      KServiceTile(
        icon: Icons.lunch_dining_outlined,
        label: context.t('home.services.food'),
        ready: foodReady,
        soonLabel: soon,
        onTap: onFood,
      ),
      KServiceTile(
        icon: Icons.shopping_basket_outlined,
        label: context.t('home.services.market'),
        ready: marketReady,
        soonLabel: soon,
        onTap: onMarket,
      ),
      KServiceTile(
        icon: Icons.inventory_2_outlined,
        label: context.t('home.services.courier'),
        ready: courierReady,
        soonLabel: soon,
        onTap: onCourier,
      ),
      KServiceTile(
        icon: Icons.handyman_outlined,
        label: context.t('home.services.services'),
        ready: false,
        soonLabel: soon,
      ),
      KServiceTile(
        icon: Icons.credit_card_outlined,
        label: context.t('home.services.payments'),
        onTap: onPayments,
      ),
    ];
    // Lartësi fikse: një pllakë ka ikonë 50 px, emër dhe ndonjëherë "së shpejti"; me raport
    // gjerësi/lartësi do të ngushtohej në telefonat 360 px dhe do të dilte jashtë.
    return GridView(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        mainAxisSpacing: 10,
        crossAxisSpacing: 10,
        mainAxisExtent: 140,
      ),
      children: tiles,
    );
  }
}

class _RideRow extends StatelessWidget {
  const _RideRow({required this.ride, required this.locale});

  final Ride ride;
  final String locale;

  @override
  Widget build(BuildContext context) {
    final where = ride.dropoffAddress ?? formatDistance(ride.distanceM, locale: locale);
    return KCard(
      padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: K.surface2,
              borderRadius: BorderRadius.circular(K.rSm),
              border: Border.all(color: K.line),
            ),
            child: const Icon(Icons.directions_car_outlined, size: 18, color: K.textDim),
          ),
          const SizedBox(width: K.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  where,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: K.text),
                ),
                const SizedBox(height: 2),
                Text(
                  context.t(rideStateKey(ride.state)),
                  style: const TextStyle(fontSize: 12, color: K.muted),
                ),
              ],
            ),
          ),
          const SizedBox(width: K.s3),
          KMoney(
            ride.priceMinor,
            locale: locale,
            size: 16,
            strikethrough: ride.state == RideState.cancelled,
          ),
        ],
      ),
    );
  }
}
