import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';
import 'food/discover.dart';
import 'market/market.dart';
import 'food/order_tracking.dart';
import 'notifications.dart';
import 'parcels/new_parcel.dart';
import 'services/new_request.dart';
import 'services/request_tracking.dart';
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

/// Ballina është vetëm pikënisje: përshëndetja, çka është në rrjedhë tani, dhe hyrja te secili
/// shërbim. Nuk mban përmbajtjen e asnjë shërbimi — as restorante, as histori, as kërkim që në
/// të vërtetë do të thotë "udhëtim". Secili shërbim e ka ekranin e vet të plotë me kërkimin,
/// gjendjet dhe historinë e veta; historia e përbashkët jeton te skeda Aktiviteti.
///
/// Kjo është arkitektura e Gojek-ut, Grab-it dhe Careem-it, dhe arsyeja është e thjeshtë: sapo
/// ballina nis të mbajë përmbajtje të një shërbimi, të tjerët e kërkojnë të njëjtën gjë dhe
/// asnjë prej tyre nuk mbetet i përdorshëm.
class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  Future<void> _refresh() => context.read<AppState>().refreshHome();

  void _open(Widget screen) {
    Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => screen));
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final me = state.me;
    final active = state.activeRide;
    final activeOrder = state.activeOrder;
    final activeParcel = state.activeParcel;
    final activeService = state.activeService;

    return SafeArea(
      child: RefreshIndicator(
        onRefresh: _refresh,
        color: K.brand500,
        backgroundColor: K.surface2,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, K.s8),
          children: [
            _Header(name: me?.displayName ?? '—'),
            // Çka është në rrjedhë rri para gjithçkaje: kur ke një udhëtim në ecje, asgjë tjetër
            // në këtë ekran nuk të intereson.
            if (active != null) ...[
              const SizedBox(height: K.s4),
              KNeonBanner(
                icon: Icons.directions_car_outlined,
                title: context.t('home.active.ride'),
                subtitle: _rideSubtitle(context, active),
                onTap: () => _open(TrackingScreen(rideId: active.id)),
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
            if (activeParcel != null) ...[
              const SizedBox(height: K.s4),
              KNeonBanner(
                icon: Icons.inventory_2_outlined,
                title: context.t('parcel.active'),
                subtitle: context.t(parcelStateKey(activeParcel.state)),
                onTap: () => _open(ParcelTrackingScreen(parcelId: activeParcel.id)),
              ),
            ],
            if (activeService != null) ...[
              const SizedBox(height: K.s4),
              KNeonBanner(
                icon: Icons.handyman_outlined,
                title: context.t('service.active'),
                subtitle: context.t(serviceStateKey(activeService.state)),
                onTap: () => _open(ServiceTrackingScreen(requestId: activeService.id)),
              ),
            ],
            const SizedBox(height: K.s4),
            KHeroCarousel(slides: _slides(context)),
            const SizedBox(height: K.s4),
            _ServicesGrid(
              rideReady: state.config.flag('rides', fallback: true),
              foodReady: state.config.flag('food'),
              marketReady: state.config.flag('market', fallback: true),
              courierReady: state.config.flag('parcels', fallback: true),
              servicesReady: state.config.flag('services', fallback: true),
              onRide: () => _open(
                active == null ? const DestinationScreen() : TrackingScreen(rideId: active.id),
              ),
              onFood: () => _open(const DiscoverScreen()),
              onMarket: () => _open(const MarketScreen()),
              onServices: () => _open(
                activeService == null
                    ? const NewServiceRequestScreen()
                    : ServiceTrackingScreen(requestId: activeService.id),
              ),
              onCourier: () => _open(
                activeParcel == null
                    ? const NewParcelScreen()
                    : ParcelTrackingScreen(parcelId: activeParcel.id),
              ),
              onPayments: () => _open(const WalletScreen()),
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
  /// Karuseli tregon vetë shërbimet me fotot e paketuara. Kopertinat e restoranteve i takojnë
  /// ekranit të Ushqimit: në ballinë ato do të ishin përsëri një shërbim që fle në shtratin e
  /// të tjerëve. Asnjë shifër dhe asnjë ofertë e shpikur — vetëm ajo që aplikacioni bën vërtet.
  List<KHeroSlide> _slides(BuildContext context) {
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
}

class _Header extends StatelessWidget {
  const _Header({required this.name});

  final String name;

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
      const SizedBox(width: K.s2),
      const NotificationsButton(),
    ],
  );
}

class _ServicesGrid extends StatelessWidget {
  const _ServicesGrid({
    required this.rideReady,
    required this.foodReady,
    required this.marketReady,
    required this.courierReady,
    required this.servicesReady,
    required this.onRide,
    required this.onFood,
    required this.onMarket,
    required this.onCourier,
    required this.onServices,
    required this.onPayments,
  });

  final bool rideReady;
  final bool foodReady;
  final bool marketReady;
  final bool courierReady;
  final bool servicesReady;
  final VoidCallback onRide;
  final VoidCallback onFood;
  final VoidCallback onMarket;
  final VoidCallback onCourier;
  final VoidCallback onServices;
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
        ready: servicesReady,
        soonLabel: soon,
        onTap: onServices,
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
