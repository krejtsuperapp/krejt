import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:krejt_api/krejt_api.dart';

import '../services/location.dart';

/// Turni i punës: sa është shoferi në punë, aplikacioni dërgon pozicionin, pyet për oferta
/// dhe ndjek udhëtimin aktiv. Jashtë pune asgjë nga këto nuk ndodh — as një kërkesë e vetme (§27).
class WorkState extends ChangeNotifier {
  WorkState({required this.api, this.location = const LocationService()});

  static const _offersEvery = Duration(seconds: 3);
  static const _locationEvery = Duration(seconds: 10);
  static const _rideEvery = Duration(seconds: 4);

  final KrejtApi api;
  final LocationService location;

  Timer? _offersTimer;
  Timer? _locationTimer;
  Timer? _rideTimer;

  bool online = false;
  bool busy = false;
  List<RideOffer> offers = const [];
  Ride? activeRide;
  ApiError? lastError;
  LocationProblem? locationProblem;

  RideOffer? get topOffer {
    for (final o in offers) {
      if (!o.expired) return o;
    }
    return null;
  }

  @override
  void dispose() {
    _stopTimers();
    super.dispose();
  }

  void _stopTimers() {
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
    // Gjatë një udhëtimi aktiv nuk kërkohen oferta të reja: shoferi është i zënë.
    if (activeRide != null) {
      if (offers.isNotEmpty) {
        offers = const [];
        notifyListeners();
      }
      return;
    }
    try {
      final items = await api.driverOffers();
      offers = items;
      lastError = null;
      notifyListeners();
    } on ApiError catch (e) {
      lastError = e;
      notifyListeners();
    }
  }

  Future<void> _pollActiveRide() async {
    try {
      final ride = await api.driverActiveRide();
      final changed = ride?.id != activeRide?.id || ride?.state != activeRide?.state;
      activeRide = ride;
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
