import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:krejt_api/krejt_api.dart';

import '../services/location.dart';

/// Turni i punës: sa është shoferi në punë, aplikacioni dërgon pozicionin, pyet për oferta
/// dhe ndjek udhëtimin aktiv. Jashtë pune asgjë nga këto nuk ndodh — as një kërkesë e vetme (§27).
class WorkState extends ChangeNotifier {
  WorkState({
    required this.api,
    this.location = const LocationService(),
    this.realtime,
    this.driverId,
  });

  // Rezervë: kanali i gjallë sjell ofertat dhe ndryshimet e udhëtimit menjëherë; këto e kapin
  // vetëm rastin kur ai bie. Pozicioni dërgohet gjithmonë me orar — nuk varet nga kanali.
  // Ofertat jetojnë 20 sekonda te serveri. Rezerva prej 10 s ishte e tepërt: nëse kanali i gjallë
  // vonohet, shoferi e sheh ofertën me pak sekonda mbetur dhe pranimi bie si e skaduar — kërkesa
  // 'zhduket'. Tri sekonda kushtojnë pak dhe e mbyllin këtë vrimë; pozicioni mbetet çdo 10 s.
  static const _offersEvery = Duration(seconds: 3);
  static const _locationEvery = Duration(seconds: 10);
  static const _rideEvery = Duration(seconds: 15);

  final KrejtApi api;
  final LocationService location;

  /// Kanali i gjallë dhe identiteti i shoferit; pa to, turni punon vetëm me pyetje periodike.
  final RealtimeClient? realtime;
  final String? Function()? driverId;
  StreamSubscription<RealtimeEvent>? _live;

  Timer? _offersTimer;
  Timer? _locationTimer;
  Timer? _rideTimer;

  bool online = false;
  bool busy = false;
  List<RideOffer> offers = const [];
  List<CourierOffer> deliveryOffers = const [];
  List<ParcelOffer> parcelOffers = const [];
  Ride? activeRide;
  Order? activeOrder;
  Parcel? activeParcel;
  ApiError? lastError;
  LocationProblem? locationProblem;

  RideOffer? get topOffer {
    for (final o in offers) {
      if (!o.expired) return o;
    }
    return null;
  }

  CourierOffer? get topDeliveryOffer {
    for (final o in deliveryOffers) {
      if (!o.expired) return o;
    }
    return null;
  }

  ParcelOffer? get topParcelOffer {
    for (final o in parcelOffers) {
      if (o.secondsLeft > 0) return o;
    }
    return null;
  }

  /// I zënë me punë: as udhëtim, as dorëzim, as pako e re nuk ka kuptim derisa kjo të mbarojë.
  bool get isBusyWithWork => activeRide != null || activeOrder != null || activeParcel != null;

  @override
  void dispose() {
    _stopTimers();
    super.dispose();
  }

  /// Kanali i shoferit sjell ofertat dhe ndryshimet e udhëtimit; çdo ngjarje është një shtytje
  /// për të marrë gjendjen nga serveri, që modeli të mbetet një.
  void _listenLive() {
    final id = driverId?.call();
    final rt = realtime;
    if (rt == null || id == null) return;
    _live?.cancel();
    _live = rt.subscribe(driverChannel(id)).listen((_) {
      unawaited(_pollOffers());
      unawaited(_pollActiveRide());
    });
  }

  void _stopTimers() {
    _live?.cancel();
    _live = null;
    _offersTimer?.cancel();
    _locationTimer?.cancel();
    _rideTimer?.cancel();
    _offersTimer = null;
    _locationTimer = null;
    _rideTimer = null;
  }

  /// Hyrja në punë kërkon vendndodhjen: pa të, dispeçeri nuk di ku je dhe kërkesat nuk vijnë.
  Future<bool> goOnline(List<String> categories) async {
    busy = true;
    lastError = null;
    locationProblem = null;
    notifyListeners();
    try {
      final position = await location.current();
      if (!position.isOk) {
        locationProblem = position.problem;
        return false;
      }
      await api.goOnline(categories);
      await api.pushLocation(lat: position.point!.lat, lng: position.point!.lng);
      online = true;
      _offersTimer = Timer.periodic(_offersEvery, (_) => _pollOffers());
      _locationTimer = Timer.periodic(_locationEvery, (_) => _pushLocation());
      _rideTimer = Timer.periodic(_rideEvery, (_) => _pollActiveRide());
      _listenLive();
      unawaited(_pollOffers());
      unawaited(_pollActiveRide());
      return true;
    } on ApiError catch (e) {
      lastError = e;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<void> goOffline() async {
    busy = true;
    notifyListeners();
    try {
      await api.goOffline();
    } on ApiError catch (e) {
      lastError = e;
    } finally {
      _stopTimers();
      online = false;
      offers = const [];
      deliveryOffers = const [];
      parcelOffers = const [];
      busy = false;
      notifyListeners();
    }
  }

  Future<void> _pushLocation() async {
    final position = await location.current();
    if (!position.isOk) return;
    try {
      await api.pushLocation(lat: position.point!.lat, lng: position.point!.lng);
    } on ApiError {
      // Një mostër e humbur nuk prish turnin; e radhësja niset pas dhjetë sekondash.
    }
  }

  Future<void> _pollOffers() async {
    // Gjatë një pune aktive nuk kërkohen oferta të reja: shoferi është i zënë.
    if (isBusyWithWork) {
      if (offers.isNotEmpty || deliveryOffers.isNotEmpty || parcelOffers.isNotEmpty) {
        offers = const [];
        deliveryOffers = const [];
        parcelOffers = const [];
        notifyListeners();
      }
      return;
    }
    try {
      final results = await Future.wait([
        api.driverOffers(),
        api.courierOffers(),
        api.courierParcelOffers(),
      ]);
      offers = results[0] as List<RideOffer>;
      deliveryOffers = results[1] as List<CourierOffer>;
      parcelOffers = results[2] as List<ParcelOffer>;
      lastError = null;
      notifyListeners();
    } on ApiError catch (e) {
      lastError = e;
      notifyListeners();
    }
  }

  /// Numërohet çdo ndryshim lokal i punës (pranim, hap i shoferit); një sondazh që nisi para
  /// ndryshimit sjell gjendje të vjetër dhe hidhet poshtë, që të mos fshijë udhëtimin e sapopranuar.
  int _workGen = 0;

  Future<void> _pollActiveRide() async {
    final gen = _workGen;
    try {
      final results = await Future.wait([
        api.driverActiveRide(),
        api.courierActiveOrder(),
        api.courierActiveParcel(),
      ]);
      if (gen != _workGen) return;
      final ride = results[0] as Ride?;
      final order = results[1] as Order?;
      final parcel = results[2] as Parcel?;
      final changed =
          ride?.id != activeRide?.id ||
          ride?.state != activeRide?.state ||
          order?.id != activeOrder?.id ||
          order?.state != activeOrder?.state ||
          parcel?.id != activeParcel?.id ||
          parcel?.state != activeParcel?.state;
      activeRide = ride;
      activeOrder = order;
      activeParcel = parcel;
      if (changed) notifyListeners();
    } on ApiError catch (e) {
      lastError = e;
      notifyListeners();
    }
  }

  Future<bool> accept(RideOffer offer) async {
    busy = true;
    notifyListeners();
    try {
      activeRide = await api.acceptOffer(offer.id);
      _workGen++;
      offers = const [];
      return true;
    } on ApiError catch (e) {
      lastError = e;
      // Oferta mund të jetë marrë nga dikush tjetër; lista rifreskohet menjëherë.
      unawaited(_pollOffers());
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<void> decline(RideOffer offer) async {
    offers = offers.where((o) => o.id != offer.id).toList();
    notifyListeners();
    try {
      await api.declineOffer(offer.id);
    } on ApiError {
      // Refuzimi është i pakthyeshëm nga ana e klientit; serveri e kalon te shoferi tjetër.
    }
  }

  // ------------------------------------------------------------------ dorëzimi

  Future<bool> acceptDelivery(CourierOffer offer) async {
    busy = true;
    notifyListeners();
    try {
      activeOrder = await api.acceptCourierOffer(offer.id);
      _workGen++;
      deliveryOffers = const [];
      return true;
    } on ApiError catch (e) {
      lastError = e;
      unawaited(_pollOffers());
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<void> declineDelivery(CourierOffer offer) async {
    deliveryOffers = deliveryOffers.where((o) => o.id != offer.id).toList();
    notifyListeners();
    try {
      await api.declineCourierOffer(offer.id);
    } on ApiError {
      // Serveri e kalon te korrieri tjetër; klienti nuk ka çfarë të rregullojë.
    }
  }

  Future<bool> pickup({required String code}) =>
      _orderStep(() => api.courierPickup(activeOrder!.id, code: code));

  Future<bool> deliver() => _orderStep(() => api.courierDeliver(activeOrder!.id));

  Future<bool> release({String? reason}) =>
      _orderStep(() => api.courierRelease(activeOrder!.id, reason: reason));

  Future<bool> _orderStep(Future<Order> Function() run) async {
    if (activeOrder == null) return false;
    busy = true;
    lastError = null;
    notifyListeners();
    try {
      final order = await run();
      // Dorëzimi i kryer ose i lëshuar e liron korrierin për punën e radhës.
      activeOrder = (order.isActive && order.courierId != null) ? order : null;
      return true;
    } on ApiError catch (e) {
      lastError = e;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  // ------------------------------------------------------------------ pakot

  Future<bool> acceptParcel(ParcelOffer offer) async {
    busy = true;
    notifyListeners();
    try {
      activeParcel = await api.acceptParcelOffer(offer.id);
      _workGen++;
      parcelOffers = const [];
      return true;
    } on ApiError catch (e) {
      lastError = e;
      unawaited(_pollOffers());
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<void> declineParcel(ParcelOffer offer) async {
    parcelOffers = parcelOffers.where((o) => o.id != offer.id).toList();
    notifyListeners();
    try {
      await api.declineParcelOffer(offer.id);
    } on ApiError {
      // Serveri e kalon te korrieri tjetër.
    }
  }

  Future<bool> pickupParcel({required String code}) =>
      _parcelStep(() => api.courierParcelPickup(activeParcel!.id, code: code));

  Future<bool> deliverParcel({required String code}) =>
      _parcelStep(() => api.courierParcelDeliver(activeParcel!.id, code: code));

  Future<bool> releaseParcel({String? reason}) =>
      _parcelStep(() => api.courierParcelRelease(activeParcel!.id, reason: reason));

  Future<bool> _parcelStep(Future<Parcel> Function() run) async {
    if (activeParcel == null) return false;
    busy = true;
    lastError = null;
    notifyListeners();
    try {
      final parcel = await run();
      _workGen++;
      activeParcel = (parcel.isActive && parcel.courierId != null) ? parcel : null;
      return true;
    } on ApiError catch (e) {
      lastError = e;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  // ------------------------------------------------------------------ udhëtimi

  Future<bool> arrived() => _step(() => api.driverArrived(activeRide!.id));

  Future<bool> start({required String pickupCode}) =>
      _step(() => api.driverStart(activeRide!.id, pickupCode: pickupCode));

  Future<bool> complete() => _step(() => api.driverComplete(activeRide!.id));

  Future<bool> cancelRide({String? reason}) =>
      _step(() => api.driverCancel(activeRide!.id, reason: reason));

  Future<bool> _step(Future<Ride> Function() run) async {
    if (activeRide == null) return false;
    busy = true;
    lastError = null;
    notifyListeners();
    try {
      final ride = await run();
      _workGen++;
      activeRide = ride.isFinished ? null : ride;
      return true;
    } on ApiError catch (e) {
      lastError = e;
      return false;
    } finally {
      busy = false;
      notifyListeners();
    }
  }
}
